package agentsmd

import (
	"regexp"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// Match whole inline constructs first so their contents are not reinterpreted
// as Markdown punctuation. Offsets remain bytes, as required by CodeEditor.
var noteMarkdownTokens = regexp.MustCompile(
	`\\.|` +
		`^ {0,3}(?:\x60{3,}.*|~{3,}.*|(?:-[ 	]*){3,}$|(?:\*[ 	]*){3,}$|(?:_[ 	]*){3,}$)|` +
		`^ {0,3}(?:#{1,6}(?:[ 	]+|$)|>+[ 	]*|[-+*][ 	]+|[0-9]+[.)][ 	]+)|` +
		`\x60+[^\x60]*\x60+|!?\[[^]\n]*\]\([^\n)]*\)|` +
		`\*\*[^*]+\*\*|__[^_]+__|~~[^~]+~~|\*[^*]+\*|_[^_]+_|\[[ xX]\]|\|`)

func highlightMarkdown(_ string, line string, dst []components.CodeEditorRun) []components.CodeEditorRun {
	color := func(index int) gfx.Color {
		return []gfx.Color{gfx.RGB(128, 128, 128), gfx.RGB(204, 51, 51), gfx.RGB(51, 153, 76), gfx.RGB(178, 128, 26), gfx.RGB(51, 115, 217), gfx.RGB(166, 76, 178)}[index]
	}
	for _, span := range noteMarkdownTokens.FindAllStringIndex(line, -1) {
		token := line[span[0]:span[1]]
		if token[0] == '\\' {
			continue
		}
		shade := color(5)
		switch token[0] {
		case '`':
			shade = color(2)
		case '[', '!':
			shade = color(4)
		case '|':
			shade = gfx.RGB(128, 128, 128)
		}
		dst = append(dst, components.CodeEditorRun{Start: span[0], End: span[1], Color: shade})
	}
	return dst
}
