package ui

import (
	// DEBUG

	"fmt"
	"politicaldissidence/data"
	"politicaldissidence/searching"

	"github.com/jroimartin/gocui"
)

// State to hold applicate search
type SearchState struct {
	// term should be cleared once the results are consumed
	term   string
	result *[]searching.Link
}

// SearchAndDisplay takes an MP and performs a background search before promptingn
// for URL results
func (ui *UI) SearchAndDisplay(g *gocui.Gui, mp data.MP) {
	// g.Update(func(g *gocui.Gui) error {
	// 	return ui.log("Searching for mp "+mp.SearchTerm(), false)
	// })
	defer ui.closeModal(SEARCHING_MODAL)

	// search term, update panel with results
	links, err := ui.search(mp.SearchTerm())
	if err != nil {
		g.Update(func(g *gocui.Gui) error {
			v, _ := g.View(LOG_PANEL)
			fmt.Fprintln(v, fmt.Sprintf("Found %d links", len(links)), false)
			ui.gui = g
			ui.state.searchState.term = mp.SearchTerm()
			ui.state.searchState.result = &links
			ui.closeModal(SEARCHING_MODAL)
			return ui.toggleListUrlsPanel(g)
		})
	}
}

// search
func (ui *UI) search(term string) ([]searching.Link, error) {
	// ui.log(fmt.Sprintf("Searching term <%s>", term), false)
	// go routine to communicate links and err
	links, err := searching.UrlSearcher{}.Search(term)
	if err != nil {
		ui.log(fmt.Sprintf("Error searching <%s> %v", term, err), true)
	} else {
		ui.log(fmt.Sprintf("Found %d links", len(links)), false)
	}
	return links, err
}
