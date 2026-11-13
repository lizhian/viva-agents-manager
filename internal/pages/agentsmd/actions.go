package agentsmd

import (
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

func (p *Page) Pointer(e gfx.PointerEvent) gfx.EventResult {
	if p.modal.Open || p.tagsOpen {
		return gfx.EventResult{}
	}
	return p.editor.pointer(e)
}
func (p *Page) Scroll(e gfx.ScrollEvent) gfx.EventResult { return p.editor.scroll(e) }
func (p *Page) Shortcut(e gfx.KeyEvent) gfx.EventResult  { return p.editor.key(e) }
func (p *Page) Key(e gfx.KeyEvent) gfx.EventResult       { return p.editor.key(e) }
func (p *Page) TextInput(ctx app.TextInputContext) *app.TextInput {
	if ctx.ID == p.editor.ids["body"] || ctx.ID == p.editor.ids["title"] {
		return p.editor.textInput(ctx)
	}
	return p.fields(ctx)
}

func (p *Page) dropDestination(source, target gfx.ID) (row, string, bool) {
	if p.query != "" {
		return row{}, "", false
	}
	var src row
	found := false
	for _, r := range p.rows {
		if r.id == source || r.grip == source {
			src = r
			found = true
			break
		}
	}
	if !found {
		return src, "", false
	}
	if target == p.ids["root"] {
		return src, "", true
	}
	for _, r := range p.rows {
		if target == r.id || target == r.before {
			return src, r.ID, r.ID != src.ID
		}
	}
	return src, "", false
}
func (p *Page) canDrop(source, target gfx.ID) bool {
	_, _, ok := p.dropDestination(source, target)
	return ok
}
func (p *Page) drop(source, target gfx.ID) {
	src, before, ok := p.dropDestination(source, target)
	if !ok || !p.flush() {
		return
	}
	if err := p.store.Data.Move(src.ID, before); err != nil {
		p.result(err)
		return
	}
	p.save(true)
}
