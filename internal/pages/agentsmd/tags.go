package agentsmd

import (
	"slices"
	"strings"

	"github.com/vivaui/viva/ui"
)

type tagChoice struct{ tag, lower, selectedLabel string }

// Like motty, one field filters existing tags and accepts comma-separated new
// tags. The anchor popover stays open while tags are added or toggled.
func (p *Page) drawTagsPopover(f ui.C, anchor ui.Box, width float32) {
	ui.PopoverWith(f, anchor, ui.PopoverSpec{Width: width, Align: ui.Start, ConsumeOutsidePress: true, InitialFocusID: p.ids["tag-query"]}, &p.tagsOpen, func() {
		n := p.store.Data.Snippet(p.target)
		if n == nil {
			return
		}
		ui.Row(f).FillWidth().Gap(6).Items(ui.Center).Children(func() {
			input := ui.TextFieldID(f, p.ids["tag-query"], &p.tagQuery, "搜索或添加", "搜索或添加").Basis(0).Grow(1).Height(30)
			if input.Changed() {
				p.tagWords = tagWords(p.tagQuery.Text())
			}
			hasNew := false
			for _, tag := range p.tagWords {
				if !slices.Contains(p.tagOptions, tag) {
					hasNew = true
					break
				}
			}
			submit := ui.Button(f, "新增").Outline().Width(60).Height(30).Disabled(!hasNew).Clicked()
			if input.Submitted() || submit {
				p.addInputTags()
			}
		})
		query := strings.ToLower(strings.TrimSpace(p.tagQuery.Text()))
		ui.Scroll(f.Key("tag-options")).FillWidth().MaxHeight(max(28, f.Bounds().Height*.6-70)).Children(func() {
			ui.Flow(f).FillWidth().Gap(6).LineGap(6).Children(func() {
				count := 0
				for _, choice := range p.tagChoices {
					if !strings.Contains(choice.lower, query) {
						continue
					}
					count++
					selected := slices.Contains(n.Tags, choice.tag)
					label := choice.tag
					if selected {
						label = choice.selectedLabel
					}
					button := ui.Button(f.Key(choice.tag), label).Small().Height(28).PadX(16).MaxWidth(width - 32).Ellipsis().Tooltip(choice.tag)
					if selected {
						button = button.Secondary().PadX(8)
					} else {
						button = button.Outline()
					}
					if button.Clicked() {
						p.toggleTag(choice.tag)
					}
				}
				if count == 0 {
					ui.Text(f, "暂无标签").FontSize(12).Muted()
				}
			})
		})
	})
}
func tagWords(value string) []string {
	var tags []string
	for _, tag := range strings.Split(value, ",") {
		tag = strings.TrimSpace(tag)
		if tag != "" && !slices.Contains(tags, tag) {
			tags = append(tags, tag)
		}
	}
	return tags
}
func (p *Page) addInputTags() {
	if _, composing := p.tagEditor.Composition(); composing {
		return
	}
	n := p.store.Data.Snippet(p.target)
	if n == nil {
		return
	}
	changed := false
	for _, tag := range tagWords(p.tagQuery.Text()) {
		if !slices.Contains(n.Tags, tag) {
			n.Tags = append(n.Tags, tag)
			changed = true
		}
	}
	if changed && !p.save(false) {
		return
	}
	p.tagQuery.SetText("")
	p.tagWords = nil
	p.tagFocus = true
}
