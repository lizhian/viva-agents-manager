package agentsmd

import (
	"slices"

	"github.com/lizhian/viva-agents-manager/internal/kit"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Toolbar buttons share their geometry and tint with the Skills page.
func iconAction(f ui.C, theme gfx.Theme, id gfx.ID, label string, icon gfx.Icon) bool {
	return kit.IconButton(f, theme, kit.IconButtonSpec{ID: id, Label: label, Icon: icon})
}

func (v *editorView) declare(f ui.C, theme gfx.Theme) {
	v.slotted = false
	n := v.p.store.Data.Snippet(v.selected)
	if n == nil {
		ui.Row(f).Height(24).Children(func() {
			if iconAction(f, theme, v.ids["library"], "显示 / 隐藏片段列表", noteIcons["PanelLeft"]) {
				v.p.toggleLibrary()
			}
		})
		ui.Slot(f).Height(40)
		message := "选择片段，或新建一个指令片段。"
		if v.selected != "" {
			message = "片段已删除"
		}
		ui.Paragraph(f, message, 3).FontSize(13).Color(theme.MutedForeground)
		return
	}
	d := v.p.buffer(v.selected)
	v.titleField.Bind(d.title)
	if v.updatedAt != n.Updated || v.updatedLabel == "" {
		v.updatedAt = n.Updated
		v.updatedLabel = "更新于 " + n.Updated.Local().Format("2006-01-02 15:04")
	}
	// The title and icon-only actions share the top bar, as in motty. The action
	// group moves to its own line only when the title would have less than 120px.
	ui.Flow(f).FillWidth().PadRight(v.p.opts.TitlebarTrailing).Gap(6).LineGap(4).MovesWindow().Children(func() {
		ui.Row(f).Basis(0).Grow(1).MinWidth(160).Height(24).Items(ui.Center).Gap(6).Children(func() {
			if iconAction(f, theme, v.ids["library"], "显示 / 隐藏片段列表", noteIcons["PanelLeft"]) {
				v.p.toggleLibrary()
			}
			ui.TextFieldID(f, v.ids["title"], &v.titleField, "片段名称", "无标题").Basis(0).Grow(1).FillWidth().Height(24).PadSides(0, 8, 0, 8).FontSize(12).Bold().Color(theme.Foreground).Background(theme.Background).Border(0, theme.Background).Shadow(gfx.ShadowNone)
		})
		ui.Row(f).Height(24).Items(ui.Center).Gap(6).Shrink(0).Children(func() {
			tagButton := ui.Row(f).Size(24, 24).Radius(6).Center().HitWith(v.ids["tags"], "管理标签", gfx.HitOptions{Role: gfx.RoleButton, Cursor: gfx.PointerCursorHand}).Tooltip("管理标签").Hover(ui.Look{}.Background(theme.Hover)).Children(func() { ui.Icon(f, noteIcons["Tags"]).Size(16, 16).Color(theme.MutedForeground) })
			if tagButton.Clicked() {
				v.openTags()
			}
			v.p.drawTagsPopover(f, tagButton, min(max(v.bodyWidth/2, 340), max(1, f.Bounds().Width-24)))

			if imagePattern.MatchString(d.body.String()) {
				if iconAction(f, theme, v.ids["load-images"], "加载图片", noteIcons["Image"]) {
					v.loadImages()
				}
			}
			ui.Slot(f).Size(84, 24).Shrink(0).Draw(v.modePainter)
		})
	})
	ui.Slot(f).Height(7)
	ui.Divider(f).Background(theme.Border)
	ui.Slot(f).Height(4)
	if len(n.Tags) > 0 {
		v.declareTags(f, theme, n.Tags)
		ui.Slot(f).Height(4)
		ui.Divider(f).Background(theme.Border)
		ui.Slot(f).Height(4)
	}
	if v.mode != "preview" {
		ui.Row(f).FillWidth().Gap(0).Items(ui.Center).Children(func() {
			ui.Flow(f).Basis(356).Shrink(1).Gap(6).LineGap(2).Children(func() {
				for i, label := range formatLabels {
					icon := gfx.Icon{}
					switch i {
					case 2:
						icon = noteIcons["Italic"]
					case 10:
						icon = noteIcons["Table"]
					case 11:
						icon = noteIcons["Minus"]
					}
					id := v.formats.At(i)
					if len(icon.Commands) > 0 {
						if iconAction(f, theme, id, formatNames[i], icon) {
							v.format(i)
						}
					} else {
						if ui.Row(f).Size(24, 24).Shrink(0).Center().Radius(4).HitWith(id, formatNames[i], gfx.HitOptions{Role: gfx.RoleButton, Cursor: gfx.PointerCursorHand}).Tooltip(formatNames[i]).Hover(ui.Look{}.Background(theme.Hover)).Children(func() { ui.Text(f, label).FontSize(13).Color(theme.Foreground) }).Clicked() {
							v.format(i)
						}
					}
				}
			})
			// On narrow layouts the status yields its room to the formatting controls.
			ui.Text(f, v.updatedLabel).Basis(0).Grow(1).Ellipsis().TextEnd().FontSize(11).Color(theme.MutedForeground)
		})
		ui.Slot(f).Height(8)
	}
	if v.findOpen {
		ui.Row(f).FillWidth().Items(ui.Center).Gap(4).Children(func() {
			ui.TextFieldID(f, v.ids["find"], &v.findField, "文内查找", "文内查找").Grow(1)
			if ui.Button(f, "上一处").XSmall().Clicked() {
				v.stepFind(false)
			}
			if ui.Button(f, "下一处").XSmall().Clicked() {
				v.stepFind(true)
			}
			if ui.Button(f, "关闭查找").Ghost().XSmall().Clicked() {
				v.findOpen = false
			}
		})
		ui.Slot(f).Height(6)
	}
	v.slot = ui.Slot(f).Grow(1).FillWidth()
	v.slotted = true
}

var workspaceModes = []struct{ mode, label, icon string }{{"source", "源码", "CodeXml"}, {"split", "分栏", "Columns2"}, {"preview", "预览", "Eye"}}
var formatNames = []string{"标题", "加粗", "斜体", "行内代码", "代码块", "无序列表", "有序列表", "任务项", "引用", "链接", "表格", "分隔线"}

func (v *editorView) declareTags(f ui.C, theme gfx.Theme, tags []string) {
	if !slices.Equal(v.cachedTags, tags) {
		v.cachedTags = slices.Clone(tags)
		v.tagLabels = nil
		for _, tag := range tags {
			label := tag
			runes := []rune(tag)
			if len(runes) > 16 {
				label = string(runes[:16]) + "…"
			}
			v.tagLabels = append(v.tagLabels, label)
		}
	}
	ui.Flow(f).FillWidth().MinHeight(24).Gap(6).LineGap(4).Children(func() {
		limit := min(3, len(tags))
		if v.showTags {
			limit = len(tags)
		}
		for i := 0; i < limit; i++ {
			tag := tags[i]
			ui.Row(f.Key(tag)).Height(22).Radius(11).Background(theme.Secondary).PadSides(0, 6, 0, 8).Items(ui.Center).Gap(4).Children(func() {
				ui.Text(f, v.tagLabels[i]).FontSize(12).Color(theme.Foreground)
				if ui.Row(f).Size(14, 14).Center().Radius(3).HitWith(v.tagRemoves.At(i), tag, gfx.HitOptions{Role: gfx.RoleButton, Cursor: gfx.PointerCursorHand}).Tooltip(tag).Hover(ui.Look{}.Background(theme.Hover)).Children(func() { ui.Icon(f, noteIcons["X"]).Size(12, 12).Color(theme.MutedForeground) }).Clicked() {
					v.p.target = v.selected
					v.p.toggleTag(tag)
				}
			})
		}
		if len(tags) > 3 {
			label := "展开标签"
			if v.showTags {
				label = "收起标签"
			}
			if ui.Button(f, label).Ghost().XSmall().Clicked() {
				v.showTags = !v.showTags
			}
		}
	})
}

func (p *Page) toggleLibrary() { p.view.Hidden = !p.view.Hidden; p.persistView(); p.invalidate() }

// drawModes delegates joined corners, dividers and selected fills to Viva.
func (v *editorView) drawModes(d *ui.DrawContext) {
	for i, m := range workspaceModes {
		variant := components.ButtonOutline
		if v.mode == m.mode {
			variant = components.ButtonSecondary
		}
		v.modeOptions[i] = components.ButtonGroupOption{ID: v.ids[m.mode], Variant: variant}
	}
	first := len(d.Canvas.Hits())
	result := components.BeginButtonGroup(d.Canvas, d.Theme.WithControlHeight(24), d.Rect, v.modeOptions[:]).Paint()
	for i, m := range workspaceModes {
		x := d.Rect.X + (float32(i)+.5)*(d.Rect.Width/3) - 8
		d.Canvas.DrawIcon(noteIcons[m.icon], gfx.Rect{X: x, Y: d.Rect.Y + (d.Rect.Height-16)/2, Width: 16, Height: 16}, d.Theme.Foreground)
		for j := first; j < len(d.Canvas.Hits()); j++ {
			hit := &d.Canvas.Hits()[j]
			if hit.ID == v.ids[m.mode] {
				hit.Label = m.label
				hit.Hint = m.label
				hit.Selected = v.mode == m.mode
			}
		}
	}
	if id, ok := result.Clicked(); ok {
		for _, m := range workspaceModes {
			if id == v.ids[m.mode] {
				v.mode = m.mode
				v.p.persistView()
				break
			}
		}
	}
}
