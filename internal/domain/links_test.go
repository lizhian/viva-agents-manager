package domain

import (
	"net/url"
	"testing"
)

// TestRewriteLinks checks which destinations become clickable, and that nothing
// else in the document changes.
func TestRewriteLinks(t *testing.T) {
	for _, test := range []struct {
		name string
		body string
		want string
	}{
		{
			name: "a sibling document",
			body: "See [DEEPENING.md](DEEPENING.md).\n",
			want: "See [DEEPENING.md](https://viva-doc.invalid/DEEPENING.md).\n",
		},
		{
			name: "a nested document with a fragment and a title",
			body: "[one](refs/one.md#top \"Title\")",
			want: "[one](https://viva-doc.invalid/refs/one.md#top \"Title\")",
		},
		{
			name: "a fenced block keeps what it says",
			body: "```\n[x](y.md)\n```\n",
			want: "```\n[x](y.md)\n```\n",
		},
		{
			name: "an inline code span keeps what it says",
			body: "Write `[x](y.md)` to link.\n",
			want: "Write `[x](y.md)` to link.\n",
		},
		{
			name: "an image is not a link",
			body: "![alt](picture.md)",
			want: "![alt](picture.md)",
		},
		{
			name: "a web address is already a link",
			body: "[docs](https://example.com/a.md)",
			want: "[docs](https://example.com/a.md)",
		},
		{
			name: "a file outside the skill stays as written",
			body: "[other](../other/x.md)",
			want: "[other](../other/x.md)",
		},
		{
			name: "an absolute path stays as written",
			body: "[root](/etc/x.md)",
			want: "[root](/etc/x.md)",
		},
		{
			name: "something that is not markdown stays as written",
			body: "[picture](picture.png)",
			want: "[picture](picture.png)",
		},
		{
			name: "an escaped bracket is not a link",
			body: `\[x](y.md)`,
			want: `\[x](y.md)`,
		},
		{
			name: "two links on one line",
			body: "[a](a.md) then [b](b.md)",
			want: "[a](https://viva-doc.invalid/a.md) then [b](https://viva-doc.invalid/b.md)",
		},
		{
			name: "a document with no links",
			body: "# Title\n\nText.\n",
			want: "# Title\n\nText.\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := RewriteLinks(test.body); got != test.want {
				t.Fatalf("RewriteLinks = %q, want %q", got, test.want)
			}
		})
	}
}

// TestLinkTarget checks the way back: a rewritten address names the file it was
// made from, and nothing else does.
func TestLinkTarget(t *testing.T) {
	for _, test := range []struct {
		target string
		file   string
		ok     bool
	}{
		{"https://viva-doc.invalid/DEEPENING.md", "DEEPENING.md", true},
		{"https://viva-doc.invalid/refs/one.md", "refs/one.md", true},
		{"https://viva-doc.invalid/a%20b.md", "a b.md", true},
		{"https://example.com/a.md", "", false},
		{"https://viva-doc.invalid/", "", false},
		{"https://viva-doc.invalid/../secret.md", "", false},
		{"", "", false},
	} {
		file, ok := LinkTarget(test.target)
		if file != test.file || ok != test.ok {
			t.Fatalf("LinkTarget(%q) = %q, %v; want %q, %v", test.target, file, ok, test.file, test.ok)
		}
	}
}

// TestRewriteLinksRoundTrip checks that a rewritten document gives back the file
// the reader clicked.
func TestRewriteLinksRoundTrip(t *testing.T) {
	for _, file := range []string{"DEEPENING.md", "refs/one.md", "a%20b.md", "目录/说明.md"} {
		body := "[go](" + file + ")"
		rewritten := RewriteLinks(body)
		target := rewritten[len("[go](") : len(rewritten)-1]
		want, _ := url.PathUnescape(file)
		got, ok := LinkTarget(target)
		if !ok || got != want {
			t.Fatalf("%q round-tripped to %q, %v; want %q", file, got, ok, want)
		}
	}
}
