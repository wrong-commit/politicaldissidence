package ui

import (
	"fmt"
	"politicaldissidence/ui/panel"
	"strings"

	"github.com/jroimartin/gocui"
)

// openSearchTermsModal closes Select a URL (keeping results) and opens the term picker.
func (ui *UI) openSearchTermsModal(g *gocui.Gui) error {
	if ui.currentModal == SEARCHING_MODAL {
		return ui.log("Search already in progress", false)
	}
	if ui.currentModal == SEARCH_TERMS_MODAL {
		return nil
	}
	if ui.currentModal != LIST_URLS_MODAL {
		return ui.log("Open Select a URL before choosing a search term", true)
	}
	mp := ui.mpAt(ui.state.currentIndex)
	if mp == nil {
		return ui.log("No MP selected for term picker", true)
	}
	cfg := ui.searchTerms()
	if cfg.Len() == 0 {
		return ui.log("No search terms loaded", true)
	}

	if err := ui.closeModal(LIST_URLS_MODAL); err != nil {
		return ui.log(fmt.Sprintf("Could not close URL list modal: %v", err), true)
	}

	ui.ensureSearchPrefs()
	ui.state.termPickerIndex, _ = cfg.ClampIndex(ui.state.searchPrefs.TermIndex())
	return ui.showSearchTermsModal(g)
}

// showSearchTermsModal creates/refreshes the Select a search term modal.
func (ui *UI) showSearchTermsModal(g *gocui.Gui) error {
	mp := ui.mpAt(ui.state.currentIndex)
	if mp == nil {
		return ui.log("No MP selected for term picker", true)
	}
	cfg := ui.searchTerms()
	idx, _ := cfg.ClampIndex(ui.state.termPickerIndex)
	ui.state.termPickerIndex = idx

	rendered, err := cfg.Render(idx, *mp)
	if err != nil {
		return ui.log(fmt.Sprintf("ERROR rendering search term: %v", err), true)
	}
	body := panel.DrawSearchTermsPanel(cfg.DisplayIndex(idx), cfg.LabelAt(idx), rendered)

	lines := strings.Count(body, "\n")
	if lines < 3 {
		lines = 3
	}
	maxX, _ := g.Size()
	w := maxX * 4 / 5
	if w < 50 {
		w = 50
	}
	if w > maxX-2 {
		w = maxX - 2
	}

	if ui.currentModal == SEARCH_TERMS_MODAL {
		v, err := g.View(SEARCH_TERMS_MODAL)
		if err != nil {
			return err
		}
		title := ui.searchTermsModalTitle()
		v.Title = title
		p := panelViews[SEARCH_TERMS_MODAL]
		p.title = title
		panelViews[SEARCH_TERMS_MODAL] = p
		return ui.writeContent2(SEARCH_TERMS_MODAL, body, g)
	}

	v, err := ui.openModal(SEARCH_TERMS_MODAL, w, lines, false)
	if err != nil {
		return ui.log(fmt.Sprintf("Could not open search terms modal: %v", err), true)
	}
	title := ui.searchTermsModalTitle()
	v.Title = title
	p := panelViews[SEARCH_TERMS_MODAL]
	p.title = title
	panelViews[SEARCH_TERMS_MODAL] = p
	v.Wrap = true
	v.Editable = false
	v.Highlight = false
	return ui.writeContent2(SEARCH_TERMS_MODAL, body, g)
}

func (ui *UI) searchTermsModalTitle() string {
	cfg := ui.searchTerms()
	return fmt.Sprintf("Select a search term (%s)", cfg.DisplayIndex(ui.state.termPickerIndex))
}

// pageSearchTermsModal moves the term picker by delta (wrap) without searching yet.
func (ui *UI) pageSearchTermsModal(g *gocui.Gui, delta int) error {
	if ui.currentModal != SEARCH_TERMS_MODAL {
		return nil
	}
	cfg := ui.searchTerms()
	next, ok := cfg.WrapIndex(ui.state.termPickerIndex, delta)
	if !ok {
		return ui.log("Only one search term configured", false)
	}
	ui.state.termPickerIndex = next
	return ui.showSearchTermsModal(g)
}

// confirmSearchTermsModal commits the picker index and re-fetches page 0.
func (ui *UI) confirmSearchTermsModal(g *gocui.Gui) error {
	if ui.currentModal != SEARCH_TERMS_MODAL {
		return nil
	}
	mp := ui.mpAt(ui.state.currentIndex)
	if mp == nil {
		return ui.log("No MP selected for term change", true)
	}
	cfg := ui.searchTerms()
	idx, _ := cfg.ClampIndex(ui.state.termPickerIndex)
	ui.state.searchPrefs.termIndex = idx

	term, err := cfg.Render(idx, *mp)
	if err != nil {
		return ui.log(fmt.Sprintf("ERROR rendering search term: %v", err), true)
	}
	_ = ui.log(fmt.Sprintf("Search term → %s <%s>", cfg.DisplayIndex(idx), term), false)

	if err := ui.closeModal(SEARCH_TERMS_MODAL); err != nil {
		return ui.log(fmt.Sprintf("Could not close search terms modal: %v", err), true)
	}
	return ui.refetchURLSearch(g, term, 0)
}

// cancelSearchTermsModal closes the picker and reopens Select a URL without changing the term.
func (ui *UI) cancelSearchTermsModal(g *gocui.Gui) error {
	if ui.currentModal != SEARCH_TERMS_MODAL {
		return nil
	}
	if err := ui.closeModal(SEARCH_TERMS_MODAL); err != nil {
		return err
	}
	if ui.state.searchState == nil || ui.state.searchState.result == nil || len(*ui.state.searchState.result) == 0 {
		return nil
	}
	return ui.toggleListUrlsModal(g)
}
