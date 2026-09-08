package ui

import (
	"fmt"
	"politicaldissidence/data"
	"politicaldissidence/searching"

	"github.com/jroimartin/gocui"
)

// SearchPrefs holds session search choices that survive closing Select a URL.
type SearchPrefs struct {
	engine    searching.Engine
	termIndex int // 1 or 2
}

func (p SearchPrefs) Engine() searching.Engine {
	return p.engine.Normalize()
}

func (p SearchPrefs) TermIndex() int {
	if p.termIndex == 2 {
		return 2
	}
	return 1
}

// SearchState holds applicate search results for the open guess-URL flow.
type SearchState struct {
	// term should be cleared once the results are consumed
	term string
	// page is 0-based; UI title shows Page (page+1)
	page   int
	result *[]searching.Link
}

func (ui *UI) ensureSearchPrefs() {
	if ui.state == nil {
		return
	}
	if ui.state.searchPrefs.engine == "" {
		ui.state.searchPrefs.engine = searching.EngineBing
	}
	if ui.state.searchPrefs.termIndex != 1 && ui.state.searchPrefs.termIndex != 2 {
		ui.state.searchPrefs.termIndex = 1
	}
}

func (ui *UI) searchTermForMP(mp data.MP) string {
	ui.ensureSearchPrefs()
	if ui.state.searchPrefs.TermIndex() == 2 {
		return mp.SearchTerm2()
	}
	return mp.SearchTerm1()
}

// SearchAndDisplay takes an MP and performs a background search (page 0)
// using the session engine/term prefs before prompting for URL results.
func (ui *UI) SearchAndDisplay(g *gocui.Gui, mp data.MP) {
	ui.ensureSearchPrefs()
	term := ui.searchTermForMP(mp)
	ui.fetchSearchPage(g, term, 0, nil, 0, mp.Name())
}

// changeURLSearchPage loads page+delta via Searching modal, then reopens Select a URL.
func (ui *UI) changeURLSearchPage(g *gocui.Gui, delta int) error {
	ui.ensureSearchPrefs()
	if ui.state.searchState == nil {
		return ui.log("No search state for paging", true)
	}
	engine := ui.state.searchPrefs.Engine()
	if !engine.SupportsPaging() {
		return ui.log(fmt.Sprintf("%s does not support paging (switch to Bing with e)", engine.Label()), false)
	}
	term := ui.state.searchState.term
	if term == "" {
		return ui.log("No search term for paging", true)
	}
	nextPage := ui.state.searchState.page + delta
	if nextPage < 0 {
		return nil // already on first page
	}
	return ui.refetchURLSearch(g, term, nextPage)
}

// toggleURLSearchEngine cycles Bing ↔ DuckDuckGo and re-fetches page 0.
func (ui *UI) toggleURLSearchEngine(g *gocui.Gui) error {
	ui.ensureSearchPrefs()
	next := ui.state.searchPrefs.Engine().Next()
	ui.state.searchPrefs.engine = next
	_ = ui.log(fmt.Sprintf("Search engine → %s", next.Label()), false)

	term := ""
	if ui.state.searchState != nil {
		term = ui.state.searchState.term
	}
	if term == "" {
		return ui.log("No search term for engine toggle", true)
	}
	return ui.refetchURLSearch(g, term, 0)
}

// toggleURLSearchTerm switches SearchTerm1 ↔ SearchTerm2 and re-fetches page 0.
func (ui *UI) toggleURLSearchTerm(g *gocui.Gui) error {
	ui.ensureSearchPrefs()
	if ui.state.visible == nil || ui.state.currentIndex < 0 || ui.state.currentIndex >= len(*ui.state.visible) {
		return ui.log("No MP selected for term toggle", true)
	}
	mp := (*ui.state.visible)[ui.state.currentIndex]
	if ui.state.searchPrefs.TermIndex() == 1 {
		ui.state.searchPrefs.termIndex = 2
	} else {
		ui.state.searchPrefs.termIndex = 1
	}
	term := ui.searchTermForMP(mp)
	_ = ui.log(fmt.Sprintf("Search term → T%d <%s>", ui.state.searchPrefs.TermIndex(), term), false)
	return ui.refetchURLSearch(g, term, 0)
}

// refetchURLSearch closes Select a URL if open, shows Searching, and fetches term/page.
func (ui *UI) refetchURLSearch(g *gocui.Gui, term string, page int) error {
	if ui.currentModal == SEARCHING_MODAL {
		return ui.log("Search already in progress", false)
	}
	if term == "" {
		return ui.log("No search term", true)
	}

	priorPage := 0
	var priorCopy []searching.Link
	if ui.state.searchState != nil {
		priorPage = ui.state.searchState.page
		if ui.state.searchState.result != nil {
			priorCopy = append([]searching.Link(nil), (*ui.state.searchState.result)...)
		}
	}

	if ui.currentModal == LIST_URLS_MODAL {
		if err := ui.closeModal(LIST_URLS_MODAL); err != nil {
			return ui.log(fmt.Sprintf("Could not close URL list modal: %v", err), true)
		}
	}
	if _, err := ui.openSearchingModal(g); err != nil {
		return ui.log(fmt.Sprintf("Could not open searching modal: %v", err), true)
	}

	ui.ensureSearchPrefs()
	engine := ui.state.searchPrefs.Engine()
	_ = ui.log(fmt.Sprintf("Searching %s page %d for <%s>", engine.Label(), page+1, term), false)

	var priorPtr *[]searching.Link
	if len(priorCopy) > 0 {
		priorPtr = &priorCopy
	}
	go ui.fetchSearchPage(g, term, page, priorPtr, priorPage, "")
	return nil
}

// fetchSearchPage runs SearchPage off the UI thread, then updates modals on the main loop.
// On failure, priorLinks/priorPage (when priorLinks != nil) restore the previous Select a URL page.
func (ui *UI) fetchSearchPage(g *gocui.Gui, term string, page int, priorLinks *[]searching.Link, priorPage int, mpName string) {
	ui.ensureSearchPrefs()
	engine := ui.state.searchPrefs.Engine()
	links, err := searching.UrlSearcher{}.SearchPage(term, page, engine)

	g.Update(func(g *gocui.Gui) error {
		if cerr := ui.closeModal(SEARCHING_MODAL); cerr != nil {
			_ = ui.log(fmt.Sprintf("Could not close searching modal: %v", cerr), true)
		}

		if err != nil {
			_ = ui.log(fmt.Sprintf("Error searching %s page %d for <%s>: %v", engine.Label(), page+1, term, err), true)
			return ui.reopenURLListAfterPageFailure(g, priorLinks, priorPage)
		}
		if len(links) == 0 {
			_ = ui.log(fmt.Sprintf("No links on %s page %d for <%s>", engine.Label(), page+1, term), true)
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
// Does not clear session SearchPrefs (engine / term index).
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

// urlListModalTitle builds the Select a URL title with page, engine, and term index.
func (ui *UI) urlListModalTitle() string {
	ui.ensureSearchPrefs()
	page := 1
	if ui.state.searchState != nil {
		page = ui.state.searchState.page + 1
		if page < 1 {
			page = 1
		}
	}
	return fmt.Sprintf("Select a URL (Page %d · %s · T%d)",
		page, ui.state.searchPrefs.Engine().Short(), ui.state.searchPrefs.TermIndex())
}
