package domain

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// Document is one skill's SKILL.md: what it declares about itself, and its body.
type Document struct {
	// Name is what the declaration block calls the skill, or its directory name
	// when it declares none.
	Name string
	// Description is the declaration block's description, empty when it has none.
	Description string
	// Body is the markdown after the declaration block.
	Body string
}

// declaration is the block a SKILL.md may open with.
type declaration struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// ParseDocument reads a SKILL.md: an optional block of YAML between two "---"
// lines, then the body.
//
// A block that does not parse is treated as absent, so a document whose YAML is
// broken still reads as text. A block with no closing line is not a block at
// all: the whole file is the body.
func ParseDocument(data []byte, fallbackName string) Document {
	text := strings.ReplaceAll(strings.TrimPrefix(string(data), "\ufeff"), "\r\n", "\n")
	document := Document{Name: fallbackName, Body: text}
	block, body, ok := splitDeclaration(text)
	if !ok {
		return document
	}
	document.Body = strings.TrimLeft(body, "\n")
	var declared declaration
	if yaml.Unmarshal([]byte(block), &declared) != nil {
		return document
	}
	if name := strings.TrimSpace(declared.Name); name != "" {
		document.Name = name
	}
	document.Description = strings.TrimSpace(declared.Description)
	return document
}

// splitDeclaration cuts the leading "---" block off a document, reporting
// whether there was one to cut.
func splitDeclaration(text string) (block, body string, ok bool) {
	lines := strings.Split(text, "\n")
	if strings.TrimSpace(lines[0]) != "---" {
		return "", "", false
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.Join(lines[1:i], "\n"), strings.Join(lines[i+1:], "\n"), true
		}
	}
	return "", "", false
}
