package main

import (
	"bytes"
	_ "embed"
	"image"
	"image/draw"
	"image/png"
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"

	"github.com/lizhian/viva-agents-manager/internal/domain"
	"github.com/lizhian/viva-agents-manager/internal/library"
	"github.com/lizhian/viva-agents-manager/internal/shell"
	"github.com/lizhian/viva-agents-manager/internal/skillinstall"
)

//go:embed assets/app-icon.png
var appIconPNG []byte

func main() {
	window := shell.New(openStore())
	config := window.Config()
	icon, err := loadAppIcon()
	if err != nil {
		log.Fatal(err)
	}
	config.Icon = icon
	var canvas gfx.Canvas
	if err := app.RunCanvas(config, &canvas); err != nil {
		log.Fatal(err)
	}
}

// openStore opens the skill library under the application's own folder. Failing
// to read a folder is not a reason to refuse to open the window, so the in-memory
// store stands in and the reason goes to the log.
func openStore() domain.Store {
	store, err := library.Open(skillinstall.Npx{})
	if err != nil {
		log.Printf("打开技能库失败，改用内存存储: %v", err)
		return domain.NewMemory()
	}
	return store
}

// loadAppIcon supplies straight RGBA pixels to Viva for unbundled development
// runs. Packaged applications use the same PNG through viva.toml.
func loadAppIcon() (app.WindowIcon, error) {
	source, err := png.Decode(bytes.NewReader(appIconPNG))
	if err != nil {
		return app.WindowIcon{}, err
	}
	bounds := source.Bounds()
	pixels := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(pixels, pixels.Bounds(), source, bounds.Min, draw.Src)
	return app.WindowIcon{Size: bounds.Dx(), RGBA: pixels.Pix}, nil
}
