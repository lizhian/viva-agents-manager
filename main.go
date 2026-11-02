package main

import (
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"

	"github.com/lizhian/viva-agents-manager/internal/domain"
	"github.com/lizhian/viva-agents-manager/internal/shell"
)

func main() {
	window := shell.New(domain.NewMemory())
	var canvas ui.Canvas
	if err := app.RunCanvas(window.Config(), &canvas); err != nil {
		log.Fatal(err)
	}
}
