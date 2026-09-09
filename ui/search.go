package ui

import (
	"fmt"
	"os"
	"politicaldissidence/data"
	"politicaldissidence/searching"
	"politicaldissidence/searchterms"

	"github.com/jroimartin/gocui"
)

// SearchPrefs holds session search choices that survive closing Select a URL.
type SearchPrefs struct {
	engine    searching.Engine
	termIndex int // 0-based index into search term config
}

func (p SearchPrefs) Engine() searching.Engine {
	return p.engine.Normalize()
}

func (p SearchPrefs) TermIndex() int {
	if p.termIndex < 0 {
		return 0
	}
	return p.termIndex
}

// SearchState holds applicate search results for the open guess-URL flow.
type SearchState struct {
	// term should be cleared once the results are consumed
	term string
	// page is 0-based ([/] paging); not shown in the modal title
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
	if ui.state.searchPrefs.termIndex < 0 {
		ui.state.searchPrefs.termIndex = 0
	}
}

func (ui *UI) searchTerms() *searchterms.Config {
	if ui.state != nil && ui.state.searchTerms != nil {
		return ui.state.searchTerms
	}
	return searchterms.Builtin()
}

// loadSearchTerms loads search_terms.json for this guess-URL flow (or built-ins).
func (ui *UI) loadSearchTerms() {
	ui.ensureSearchPrefs()
	cfg, err := searchterms.LoadFile(searchterms.DefaultConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			_ = ui.log(fmt.Sprintf("No %s; using built-in search terms", searchterms.DefaultConfigPath), false)
		} else {
			_ = ui.log(fmt.Sprintf("ERROR search terms: %v; using built-in search terms", err), true)
		}
		ui.state.searchTerms = searchterms.Builtin()
	} else {
		ui.state.searchTerms = cfg
		_ = ui.log(fmt.Sprintf("Loaded %d search terms from %s", cfg.Len(), searchterms.DefaultConfigPath), false)
	}

	clamped, changed := ui.state.searchTerms.ClampIndex(ui.state.searchPrefs.termIndex)
	if changed {
		_ = ui.log(fmt.Sprintf("Search term index clamped to T%d/%d", clamped+1, ui.state.searchTerms.Len()), false)
	}
	ui.state.searchPrefs.termIndex = clamped
}

func (ui *UI) searchTermForMP(mp data.MP) string {
	ui.ensureSearchPrefs()
	cfg := ui.searchTerms()
	term, err := cfg.Render(ui.state.searchPrefs.TermIndex(), mp)
	if err != nil {
		_ = ui.log(fmt.Sprintf("ERROR rendering search term: %v", err), true)
		return mp.SearchTerm1()
	}
	return term
}

// SearchAndDisplay takes an MP and performs a background search (page 0)
// using the session engine/term prefs before prompting for URL results.
func (ui *UI) SearchAndDisplay(g *gocui.Gui, mp data.MP) {
	ui.loadSearchTerms()
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
		return ui.log("Already on first page", false)
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

// changeURLSearchTerm cycles the configured search term by delta (wrap) and re-fetches page 0.
// Prefer openSearchTermsModal for interactive picking; this remains for programmatic use.
func (ui *UI) changeURLSearchTerm(g *gocui.Gui, delta int) error {
	ui.ensureSearchPrefs()
	mp := ui.mpAt(ui.state.currentIndex)
	if mp == nil {
		return ui.log("No MP selected for term change", true)
	}
	cfg := ui.searchTerms()
	next, ok := cfg.WrapIndex(ui.state.searchPrefs.TermIndex(), delta)
	if !ok {
		return ui.log("Only one search term configured", false)
	}
	ui.state.searchPrefs.termIndex = next
	term := ui.searchTermForMP(*mp)
	_ = ui.log(fmt.Sprintf("Search term → %s <%s>", cfg.DisplayIndex(next), term), false)
	return ui.refetchURLSearch(g, term, 0)
}

// refetchURLSearch closes Select a URL / term picker if open, shows Searching, and fetches term/page.
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

	switch ui.currentModal {
	case LIST_URLS_MODAL:
		if err := ui.closeModal(LIST_URLS_MODAL); err != nil {
			return ui.log(fmt.Sprintf("Could not close URL list modal: %v", err), true)
		}
	case SEARCH_TERMS_MODAL:
		if err := ui.closeModal(SEARCH_TERMS_MODAL); err != nil {
			return ui.log(fmt.Sprintf("Could not close search terms modal: %v", err), true)
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
// When DuckDuckGo fails and Bing succeeds, session prefs switch to Bing so later guesses keep working.
func (ui *UI) fetchSearchPage(g *gocui.Gui, term string, page int, priorLinks *[]searching.Link, priorPage int, mpName string) {
	ui.ensureSearchPrefs()
	engine := ui.state.searchPrefs.Engine()
	links, used, err := searching.UrlSearcher{}.SearchPage(term, page, engine)

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

		if used != engine && used == searching.EngineBing {
			ui.state.searchPrefs.engine = searching.EngineBing
			_ = ui.log(fmt.Sprintf("%s failed; fell back to Bing", engine.Label()), false)
		}

		ui.state.searchState.term = term
		ui.state.searchState.page = page
		ui.state.searchState.result = &links
		label := term
		if mpName != "" {
			label = mpName
		}
		_ = ui.log(fmt.Sprintf("Found %d links for <%s> (page %d · %s)", len(links), label, page+1, used.Short()), false)
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

// urlListModalTitle builds the Select a URL title with engine and term index
// (no pagination; the active search string is shown in the modal body).
func (ui *UI) urlListModalTitle() string {
	ui.ensureSearchPrefs()
	cfg := ui.searchTerms()
	return fmt.Sprintf("Select a URL (%s · %s)",
		ui.state.searchPrefs.Engine().Short(), cfg.DisplayIndex(ui.state.searchPrefs.TermIndex()))
}
