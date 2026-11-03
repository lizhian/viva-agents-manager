// Package icons holds the vector glyphs the window draws, compiled from the
// Lucide set (<https://lucide.dev>) ahead of time so no SVG parser ships in the
// application.
//
// To add a glyph, save its SVG from lucide.dev into svg/ and run
// `go generate ./internal/icons`. icons.go is generated from those sources: edit
// an SVG, never the Go file.
//
// A glyph is drawn as triangles, which `viva dev` does not carry when it replays
// a frame, so an icon is invisible there. `go run` and a built program draw it.
package icons

//go:generate viva icons -o icons.go -pkg icons -license LICENSE svg
