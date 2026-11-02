package domain

import (
	"net/url"
	"path"
	"strings"
)

// linkHost is the host a skill's own links are rewritten to. The ".invalid"
// top-level domain is reserved (RFC 2606), so no real address can collide with
// it and the renderer keeps the link.
const linkHost = "viva-doc.invalid"

// RewriteLinks makes the local links of a skill document absolute, so the
// renderer keeps them as links instead of dropping the destination.
//
// A destination is rewritten when it names a markdown file: no scheme, no host,
// no leading slash, and a ".md" end. Everything else, and every link inside
// code, is left exactly as it is — code shows what it says.
func RewriteLinks(body string) string {
	lines := strings.Split(body, "\n")
	fence := ""
	for i, line := range lines {
		if fence != "" {
			if closesFence(line, fence) {
				fence = ""
			}
			continue
		}
		if opened := openedFence(line); opened != "" {
			fence = opened
			continue
		}
		lines[i] = rewriteLine(line)
	}
	return strings.Join(lines, "\n")
}

// LinkTarget reports whether target is one RewriteLinks produced, and the file
// it names, relative to the skill's directory and written with forward slashes.
func LinkTarget(target string) (string, bool) {
	parsed, err := url.Parse(target)
	if err != nil || parsed.Host != linkHost {
		return "", false
	}
	file := strings.TrimPrefix(parsed.Path, "/")
	if file == "" || strings.Contains(file, "..") {
		return "", false
	}
	return file, true
}

// rewriteLine rewrites the links of one line that is not inside a fence.
func rewriteLine(line string) string {
	if !strings.Contains(line, "](") {
		return line
	}
	var out strings.Builder
	out.Grow(len(line))
	written := 0
	for i := 0; i < len(line); {
		switch line[i] {
		case '\\':
			i += 2
		case '`':
			// A code span shows what it says, so a link in one is text.
			n := runLength(line, i, '`')
			if end := closingRun(line, i+n, n); end >= 0 {
				i = end + n
			} else {
				i += n
			}
		case '[':
			if i > 0 && line[i-1] == '!' {
				// An image: the renderer shows it as written.
				i++
				continue
			}
			found, ok := findLinkAt(line, i)
			if !ok {
				i++
				continue
			}
			file, fragment, local := localMarkdown(found.destination)
			if !local {
				i = found.end
				continue
			}
			out.WriteString(line[written:found.destStart])
			out.WriteString(linkURL(file, fragment))
			written = found.destEnd
			i = found.end
		default:
			i++
		}
	}
	if written == 0 {
		return line
	}
	out.WriteString(line[written:])
	return out.String()
}

// localMarkdown reports whether destination names a markdown file in the skill,
// and returns the path and fragment it names.
func localMarkdown(destination string) (file, fragment string, ok bool) {
	parsed, err := url.Parse(destination)
	if err != nil || parsed.Scheme != "" || parsed.Host != "" || parsed.Path == "" {
		return "", "", false
	}
	if strings.HasPrefix(parsed.Path, "/") || !strings.EqualFold(path.Ext(parsed.Path), ".md") {
		return "", "", false
	}
	if strings.Contains(parsed.Path, "..") {
		// A file outside the skill is not one the reader can open, so it stays as
		// written rather than becoming a link that does nothing.
		return "", "", false
	}
	return parsed.Path, parsed.Fragment, true
}

// linkURL is the absolute form of a file's path, keeping any fragment so the
// document reads the same as it was written.
func linkURL(file, fragment string) string {
	return (&url.URL{Scheme: "https", Host: linkHost, Path: "/" + file, Fragment: fragment}).String()
}

// foundLink is one [label](destination) found in a line.
type foundLink struct {
	// destination is the destination with its angle brackets removed.
	destination string
	destStart   int // where the destination begins in the line
	destEnd     int // one past where it ends
	end         int // one past the closing parenthesis
}

// findLinkAt recognizes [label](destination) at s[i], as the renderer does: the
// label is balanced brackets, and the destination runs to the closing
// parenthesis or to the blank that starts a title.
func findLinkAt(s string, i int) (foundLink, bool) {
	depth := 0
	j := i
	for ; j < len(s) && j-i <= 1000; j++ {
		switch s[j] {
		case '\\':
			j++
			continue
		case '[':
			depth++
			continue
		case ']':
			depth--
			if depth == 0 {
				goto labelEnd
			}
		}
	}
	return foundLink{}, false
labelEnd:
	if j+1 >= len(s) || s[j+1] != '(' {
		return foundLink{}, false
	}
	start := j + 2
	parens := 0
	k := start
	for ; k < len(s); k++ {
		switch s[k] {
		case '(':
			parens++
			continue
		case ')':
			if parens == 0 {
				goto destinationEnd
			}
			parens--
			continue
		case ' ', '\t':
			goto destinationEnd
		}
	}
	return foundLink{}, false
destinationEnd:
	if k >= len(s) {
		return foundLink{}, false
	}
	found := foundLink{
		destination: strings.TrimSuffix(strings.TrimPrefix(s[start:k], "<"), ">"),
		destStart:   start,
		destEnd:     k,
	}
	if s[k] != ')' {
		// A title follows; the link ends at the next parenthesis.
		rest := s[k:min(len(s), k+400)]
		close := strings.IndexByte(rest, ')')
		if close < 0 {
			return foundLink{}, false
		}
		k += close
	}
	found.end = k + 1
	return found, true
}

// openedFence reports the run of fence characters a line opens a code block
// with, or "" when it opens none.
func openedFence(line string) string {
	trimmed, indent := trimQuoteAndIndent(line)
	if indent > 3 || trimmed == "" || (trimmed[0] != '`' && trimmed[0] != '~') {
		return ""
	}
	n := runLength(trimmed, 0, trimmed[0])
	if n < 3 {
		return ""
	}
	// ```code``` on one line is inline code, not a block.
	if trimmed[0] == '`' && strings.IndexByte(trimmed[n:], '`') >= 0 {
		return ""
	}
	return trimmed[:n]
}

// closesFence reports whether a line closes the fence opened with run.
func closesFence(line, run string) bool {
	trimmed, indent := trimQuoteAndIndent(line)
	if indent > 3 || trimmed == "" || trimmed[0] != run[0] {
		return false
	}
	n := runLength(trimmed, 0, run[0])
	return n >= len(run) && n == len(trimmed)
}

// trimQuoteAndIndent strips a block quote's markers and the indentation before
// the text, as the renderer does before it looks for a fence.
func trimQuoteAndIndent(line string) (trimmed string, indent int) {
	for strings.HasPrefix(strings.TrimLeft(line, " \t"), ">") {
		line = strings.TrimPrefix(strings.TrimLeft(line, " \t")[1:], " ")
	}
	i := 0
	for ; i < len(line); i++ {
		switch line[i] {
		case ' ':
			indent++
		case '\t':
			indent += 4
		default:
			return strings.TrimRight(line[i:], " \t"), indent
		}
	}
	return "", indent
}

// runLength is the length of the run of ch that starts at i.
func runLength(s string, i int, ch byte) int {
	n := 0
	for i+n < len(s) && s[i+n] == ch {
		n++
	}
	return n
}

// closingRun finds the run of exactly n ch that ends the code span opened after
// i, or -1 when the span is never closed.
func closingRun(s string, i, n int) int {
	for i < len(s) {
		if s[i] != '`' {
			i++
			continue
		}
		length := runLength(s, i, '`')
		if length == n {
			return i
		}
		i += length
	}
	return -1
}
