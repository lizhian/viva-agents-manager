// Package version exposes the version embedded into the packaged application.
package version

import (
	_ "embed"
	"strings"
)

//go:embed version.txt
var number string

// Label is computed once; rendering the footer does not allocate.
var Label = "v" + strings.TrimSpace(number)
