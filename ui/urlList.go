package ui

import (
	"fmt"
	"politicaldissidence/searching"
	"politicaldissidence/ui/panel"

	"github.com/jroimartin/gocui"
)

// moveURLListSelection moves the Select a URL cursor by delta items (not buffer lines).
func (ui *UI) moveURLListSelection(v *gocui.View, delta int) error {
	if v == nil || ui.state.searchState == nil || ui.state.searchState.result == nil {
		return nil
	}
	n := len(*ui.state.searchState.result)
	if n == 0 {
		return nil
	}
	_, cy := v.Cursor()
	idx := panel.URLListItemIndex(cy)
	if idx < 0 {
		idx = 0
	}
	idx += delta
	if idx < 0 {
		idx = 0
	}
	if idx >= n {
		idx = n - 1
	}
	ny := panel.URLListCursorY(idx)
	_ = v.SetCursor(0, ny)
	ui.cursors.Set(LIST_URLS_MODAL, 0, ny)
	return nil
}

// selectedURLListLink returns the link for the current Select a URL cursor position.
func (ui *UI) selectedURLListLink(v *gocui.View) (searching.Link, error) {
	if ui.state.searchState == nil || ui.state.searchState.result == nil {
		return nil, fmt.Errorf("no URL search results")
	}
	links := *ui.state.searchState.result
	if len(links) == 0 {
		return nil, fmt.Errorf("no URL search results")
	}
	_, cy := v.Cursor()
	idx := panel.URLListItemIndex(cy)
	if idx < 0 || idx >= len(links) {
		return nil, fmt.Errorf("cursor position %d is not a URL item", cy)
	}
	link := links[idx]
	if len(link) == 0 {
		return nil, fmt.Errorf("empty link at index %d", idx)
	}
	return link, nil
}
