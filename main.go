package main

import (
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"

	"github.com/lizhian/viva-agents-manager/internal/domain"
	"github.com/lizhian/viva-agents-manager/internal/library"
	"github.com/lizhian/viva-agents-manager/internal/shell"
	"github.com/lizhian/viva-agents-manager/internal/skillinstall"
)

func main() {
	window := shell.New(openStore())
	var canvas ui.Canvas
	if err := app.RunCanvas(window.Config(), &canvas); err != nil {
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
