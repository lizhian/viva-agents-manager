package domain

import "testing"

// TestParseDocument checks what a SKILL.md declares about itself, and what is
// left as its body.
func TestParseDocument(t *testing.T) {
	for _, test := range []struct {
		name     string
		source   string
		fallback string
		want     Document
	}{
		{
			name:     "a declared skill",
			source:   "---\nname: Example\ndescription: Does a thing.\n---\n# Body\n",
			fallback: "alpha",
			want:     Document{Name: "Example", Description: "Does a thing.", Body: "# Body\n"},
		},
		{
			name:     "no declaration block",
			source:   "# Body\n\nText.\n",
			fallback: "alpha",
			want:     Document{Name: "alpha", Body: "# Body\n\nText.\n"},
		},
		{
			name:     "a block nobody closed",
			source:   "---\nname: Example\n# Body\n",
			fallback: "alpha",
			want:     Document{Name: "alpha", Body: "---\nname: Example\n# Body\n"},
		},
		{
			name:     "yaml that does not parse",
			source:   "---\nname: [oops\ndescription: lost\n---\n# Body\n",
			fallback: "alpha",
			want:     Document{Name: "alpha", Body: "# Body\n"},
		},
		{
			name:     "a folded description",
			source:   "---\nname: Folded\ndescription: >\n  line one\n  line two\n---\nbody\n",
			fallback: "alpha",
			want:     Document{Name: "Folded", Description: "line one line two", Body: "body\n"},
		},
		{
			name:     "a literal description",
			source:   "---\ndescription: |\n  line one\n  line two\n---\nbody\n",
			fallback: "alpha",
			want:     Document{Name: "alpha", Description: "line one\nline two", Body: "body\n"},
		},
		{
			name:     "a byte order mark and windows line endings",
			source:   "\ufeff---\r\nname: Windows\r\n---\r\n# Body\r\n",
			fallback: "alpha",
			want:     Document{Name: "Windows", Body: "# Body\n"},
		},
		{
			name:     "a name of nothing but spaces",
			source:   "---\nname: \"  \"\ndescription: kept\n---\nbody\n",
			fallback: "alpha",
			want:     Document{Name: "alpha", Description: "kept", Body: "body\n"},
		},
		{
			name:     "a quoted name",
			source:   "---\nname: \"Quoted Name\"\n---\nbody\n",
			fallback: "alpha",
			want:     Document{Name: "Quoted Name", Body: "body\n"},
		},
		{
			name:     "an empty block",
			source:   "---\n---\nbody\n",
			fallback: "alpha",
			want:     Document{Name: "alpha", Body: "body\n"},
		},
		{
			name:     "an empty file",
			source:   "",
			fallback: "alpha",
			want:     Document{Name: "alpha", Body: ""},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := ParseDocument([]byte(test.source), test.fallback); got != test.want {
				t.Fatalf("ParseDocument = %+v, want %+v", got, test.want)
			}
		})
	}
}

// TestParseDocumentTrimsTheGapAfterTheBlock checks that the blank lines a
// document usually leaves after its declaration do not become the first thing
// the reader sees.
func TestParseDocumentTrimsTheGapAfterTheBlock(t *testing.T) {
	document := ParseDocument([]byte("---\nname: Example\n---\n\n\n# Body\n"), "alpha")
	if document.Body != "# Body\n" {
		t.Fatalf("body = %q, want the gap gone", document.Body)
	}
}
