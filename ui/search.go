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
	term   string
	result *[]searching.Link
}

// SearchAndDisplay takes an MP and performs a background search before prompting
// for URL results.
func (ui *UI) SearchAndDisplay(g *gocui.Gui, mp data.MP) {
	term := mp.SearchTerm()
	links, err := searching.UrlSearcher{}.Search(term)

	g.Update(func(g *gocui.Gui) error {
		if cerr := ui.closeModal(SEARCHING_MODAL); cerr != nil {
			_ = ui.log(fmt.Sprintf("Could not close searching modal: %v", cerr), true)
		}

		if err != nil {
			return ui.log(fmt.Sprintf("Error searching <%s>: %v", term, err), true)
		}
		if len(links) == 0 {
			return ui.log(fmt.Sprintf("No links found for <%s>", term), true)
		}

		ui.state.searchState.term = term
		ui.state.searchState.result = &links
		_ = ui.log(fmt.Sprintf("Found %d links for <%s>", len(links), mp.Name()), false)
		return ui.toggleListUrlsModal(g)
	})
}
