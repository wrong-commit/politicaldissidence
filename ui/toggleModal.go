/**
 * file for toggling modal screens
 */
package ui

import (
	"sync"

	"fmt"
	"politicaldissidence/data"
	"politicaldissidence/ui/panel"
	"strings"

	"github.com/jroimartin/gocui"
)

var (
	wg sync.WaitGroup
)

// toggleHelp toggle the help view on key pressing.
func (ui *UI) toggleHelp(g *gocui.Gui, content string) error {
	if ui.currentModal == HELP_PANEL {
		// remove key bindings in original code
		// stop modal timer ?
		return ui.closeModal(ui.currentModal)
	}
	panelHeight := strings.Count(content, "\n")
	v, err := ui.openModal(HELP_PANEL, 80, panelHeight, true)
	if err != nil {
		return err
	}
	ui.gui.Cursor = false
	v.Editor = nil
	fmt.Fprint(v, content)
	return nil
}

// toggleNewDomain toggles the new domain view.
// set height to one for text input
func (ui *UI) toggleNewDomain(g *gocui.Gui) error {
	// panelHeight := strings.Count(content, "\n")
	if ui.currentModal == ADD_DOMAIN_PANEL {
		// remove new global key bindings for modal
		// stop modal timer ?
		return ui.closeModal(ui.currentModal)
	}
	v, err := ui.openModal(ADD_DOMAIN_PANEL, 40, 1, false)
	if err != nil {
		return err
	}
	ui.gui.Cursor = false
	v.Editor = gocui.DefaultEditor
	fmt.Fprintf(v, panelViews[ADD_DOMAIN_PANEL].text)
	return nil
}

// toggleLitUrlsModal opens the LIST_URLS_MODAL to select a link from the provided inputs to be added to an MP.
// TODO: callback for when URL is selected
func (ui *UI) toggleListUrlsModal(g *gocui.Gui) error {
	// close modal if already open
	if ui.currentModal == LIST_URLS_MODAL {
		// remove new global key bindings for modal
		// stop modal timer ?
		return ui.closeModal(ui.currentModal)
	}

	var existingHosts []string
	if ui.state.visible != nil && ui.state.currentIndex >= 0 && ui.state.currentIndex < len(*ui.state.visible) {
		for _, d := range (*ui.state.visible)[ui.state.currentIndex].Domains {
			existingHosts = append(existingHosts, d.Hostname)
		}
	}

	newBufferText, _, _, displayLinks, drawErr := panel.DrawListUrlPanel(g, *ui.state.searchState.result, existingHosts)
	if drawErr != nil {
		ui.log(fmt.Sprintf("Links that could not be converted to domain,\n%s", drawErr.Error()), true)
	}
	if len(displayLinks) == 0 || strings.TrimSpace(newBufferText) == "" {
		return ui.log("No domains could be extracted from search results", true)
	}
	// Handlers index into this filtered list (same order as rendered items).
	ui.state.searchState.result = &displayLinks

	maxX, maxY := g.Size()
	modalWidth := int(float64(maxX) * 0.8)
	modalHeight := int(float64(maxY) * 0.8)
	if modalWidth < 60 {
		modalWidth = 60
	}
	if modalHeight < 12 {
		modalHeight = 12
	}
	if modalWidth > maxX-2 {
		modalWidth = maxX - 2
	}
	if modalHeight > maxY-2 {
		modalHeight = maxY - 2
	}
	if modalWidth < 20 {
		modalWidth = 20
	}
	if modalHeight < 3 {
		modalHeight = 3
	}

	ui.log(fmt.Sprintf("Opening list urls modal width size (%d,%d)", modalWidth, modalHeight), false)
	v, err := ui.openModal(LIST_URLS_MODAL, modalWidth, modalHeight, false)
	if err != nil {
		return ui.log(fmt.Sprintf("Could not open URL list modal: %v", err), true)
	}
	v.Wrap = false
	v.SelBgColor = gocui.ColorCyan
	v.SelFgColor = gocui.ColorWhite
	v.Editable = false
	v.Highlight = true
	if err := ui.writeContent2(LIST_URLS_MODAL, newBufferText, g); err != nil {
		return err
	}
	// First selectable host line (below status bar).
	cy := panel.URLListCursorY(0)
	_ = v.SetCursor(0, cy)
	ui.cursors.Set(LIST_URLS_MODAL, 0, cy)
	return nil
}

// toggleListUrlsPanel to hide logo_panel and show this one
func (ui *UI) toggleListUrlsPanel(g *gocui.Gui) error {
	// newBufferText, newBufferWidth, newBufferHeight, drawErr := panel.DrawListUrlPanel(g, ui.state.searchState.result)
	// if drawErr != nil {
	// 	ui.log(fmt.Sprintf("Links that could not be converted to domain,\n%s", drawErr.Error()), true)
	// }

	// ui.log(fmt.Sprintf("Opening list urls panel width size (%d,%d)", newBufferWidth, newBufferHeight), false)
	MainViews = []string{
		LIST_URLS_PANEL,
		LOG_PANEL,
		LIST_PANEL,
		DOMAIN_PANEL,
	}
	// ui.DeleteView(LOGO_PANEL)
	// MainViews = append(MainViews, LIST_URLS_PANEL)
	// v,err := ui.gui.View(LIST_URLS_PANEL)
	// if err !=nil {
	// 	panelProperties[LIST_URLS_PANEL]
	// }
	// v.Wrap = false
	// v.SelBgColor = gocui.ColorCyan
	// v.SelFgColor = gocui.ColorWhite
	// v.Editable = false
	// v.Highlight = true
	// return ui.writeContent2(LIST_URLS_PANEL, newBufferText, g)
	return nil
}

// toggleSearchingModal opens the SEARCHING_MODAL to display while running background task
// when a URL is selected, it then opens LIST_URLS_MODAL/toggleListUrlsModal.
// when SEARCHING_MODAL is open, the screen is not interactable. after the LIST_URLS_MODAL is
// opened a URL can be selected
// TODO: support selecting multiple URLs
// TODO: custom help text
func (ui *UI) toggleSearchingModal(g *gocui.Gui) error {
	if ui.currentModal == SEARCHING_MODAL {
		// clear serch state
		ui.state.searchState.term = ""
		return ui.closeModal(ui.currentModal)
	}
	v, err := ui.openModal(SEARCHING_MODAL, 40, 1, false)
	if err != nil {
		return err
	}
	v.Editor = gocui.DefaultEditor
	ui.gui.Cursor = false
	var mp data.MP
	if ui.state.currentIndex > len(*ui.state.visible)-1 {
		return nil
	}
	mp = (*ui.state.visible)[ui.state.currentIndex]
	ui.log("Searching MP "+mp.Name(), false)
	go ui.SearchAndDisplay(g, mp)
	return nil
}
