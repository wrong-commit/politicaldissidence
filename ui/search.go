package ui

import (
	"fmt"
	"politicaldissidence/data"
	"politicaldissidence/searching"

	"github.com/jroimartin/gocui"
)

// State to hold applicate search
type SearchState struct {
	// term should be cleared once the results are consumed
	term string
	// page is 0-based; UI title shows Page (page+1)
	page   int
	result *[]searching.Link
}

// SearchAndDisplay takes an MP and performs a background Bing search (page 0)
// before prompting for URL results.
func (ui *UI) SearchAndDisplay(g *gocui.Gui, mp data.MP) {
	term := mp.SearchTerm()
	ui.fetchBingPage(g, term, 0, nil, 0, mp.Name())
}

// changeURLSearchPage loads page+delta via Searching modal, then reopens Select a URL.
func (ui *UI) changeURLSearchPage(g *gocui.Gui, delta int) error {
	if ui.state.searchState == nil {
		return ui.log("No search state for paging", true)
	}
	term := ui.state.searchState.term
	if term == "" {
		return ui.log("No search term for paging", true)
	}
	nextPage := ui.state.searchState.page + delta
	if nextPage < 0 {
		return nil // ui.log("Already on first page", false)
	}
	if ui.currentModal == SEARCHING_MODAL {
		return ui.log("Search already in progress", false)
	}

	priorPage := ui.state.searchState.page
	var priorCopy []searching.Link
	if ui.state.searchState.result != nil {
		priorCopy = append([]searching.Link(nil), (*ui.state.searchState.result)...)
	}

	if ui.currentModal == LIST_URLS_MODAL {
		if err := ui.closeModal(LIST_URLS_MODAL); err != nil {
			return ui.log(fmt.Sprintf("Could not close URL list modal: %v", err), true)
		}
	}
	if _, err := ui.openSearchingModal(g); err != nil {
		return ui.log(fmt.Sprintf("Could not open searching modal: %v", err), true)
	}
	_ = ui.log(fmt.Sprintf("Searching Bing page %d for <%s>", nextPage+1, term), false)

	go ui.fetchBingPage(g, term, nextPage, &priorCopy, priorPage, "")
	return nil
}

// fetchBingPage runs SearchPage off the UI thread, then updates modals on the main loop.
// On failure, priorLinks/priorPage (when priorLinks != nil) restore the previous Select a URL page.
func (ui *UI) fetchBingPage(g *gocui.Gui, term string, page int, priorLinks *[]searching.Link, priorPage int, mpName string) {
	links, err := searching.UrlSearcher{}.SearchPage(term, page)

	g.Update(func(g *gocui.Gui) error {
		if cerr := ui.closeModal(SEARCHING_MODAL); cerr != nil {
			_ = ui.log(fmt.Sprintf("Could not close searching modal: %v", cerr), true)
		}

		if err != nil {
			_ = ui.log(fmt.Sprintf("Error searching Bing page %d for <%s>: %v", page+1, term, err), true)
			return ui.reopenURLListAfterPageFailure(g, priorLinks, priorPage)
		}
		if len(links) == 0 {
			_ = ui.log(fmt.Sprintf("No links on Bing page %d for <%s>", page+1, term), true)
			return ui.reopenURLListAfterPageFailure(g, priorLinks, priorPage)
		}

		ui.state.searchState.term = term
		ui.state.searchState.page = page
		ui.state.searchState.result = &links
		label := term
		if mpName != "" {
			label = mpName
		}
		_ = ui.log(fmt.Sprintf("Found %d links for <%s> (page %d)", len(links), label, page+1), false)
		if err := ui.toggleListUrlsModal(g); err != nil {
			return ui.log(fmt.Sprintf("Could not open URL list modal: %v", err), true)
		}
		return nil
	})
}

func (ui *UI) reopenURLListAfterPageFailure(g *gocui.Gui, priorLinks *[]searching.Link, priorPage int) error {
	if priorLinks == nil || len(*priorLinks) == 0 {
		return nil
	}
	ui.state.searchState.page = priorPage
	ui.state.searchState.result = priorLinks
	if err := ui.toggleListUrlsModal(g); err != nil {
		return ui.log(fmt.Sprintf("Could not reopen URL list modal: %v", err), true)
	}
	return nil
}

// resetURLSearchState clears guess-URL paging state (page, term, results).
// Call when permanently dismissing Select a URL — not when closing it to paginate.
func (ui *UI) resetURLSearchState() {
	if ui.state.searchState == nil {
		return
	}
	ui.state.searchState.page = 0
	ui.state.searchState.term = ""
	empty := []searching.Link{}
	ui.state.searchState.result = &empty
	p := panelViews[LIST_URLS_MODAL]
	p.title = "Select a URL"
	panelViews[LIST_URLS_MODAL] = p
}

// closeListUrlsModal closes Select a URL and resets paging state.
func (ui *UI) closeListUrlsModal() error {
	ui.resetURLSearchState()
	return ui.closeModal(LIST_URLS_MODAL)
}
