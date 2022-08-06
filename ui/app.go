package ui

/**
 * Expose InitApp to start a new Gui
 * Provide
 */
import (
	"fmt"
	"log"
	"politicaldissidence/data"
	"politicaldissidence/db"
	"politicaldissidence/ui/panel"
	"politicaldissidence/ui/searching"
	"strings"

	"github.com/jroimartin/gocui"
)

// InitApp initialize the CLI application.
func InitApp() {
	ui := NewUI()
	defer ui.Close()
	// create internal structures and setup views in gocui
	ui.Init()
	// load data
	if err := ui.Load(); err != nil {
		ui.log(fmt.Sprintf("Could not load MPs <%s>", err.Error()), true)
	}

	ui.started = true
	ui.log(ui.startupLog, false)
	ui.Loop()
}

// Save saves written MP and domain details to disk
func (ui *UI) Save() error {
	if err := db.WriteMps(*ui.state.visible); err != nil {
		ui.log("Could not write MPs to disk", true)
		return err
	}
	return ui.log("Saved <3", false)
}

// Reload reloads data from disk, clears state
func (ui *UI) Load() error {
	mps, err := db.ReadMps()
	if err != nil {
		log.Panicln("Could not read MPs from disk", err)
	}
	ui.state.all = &mps
	ui.state.visible = &mps
	ui.log(fmt.Sprintf("Loaded %d MPs ", len(*ui.state.visible)), false)

	if v, ok := panelViews[LIST_PANEL]; ok {
		v.text = panel.DrawListMpPanel(ui.gui, &mps)
	}

	defer ui.selectMp(0)
	return nil
}

// Reload reloads data from disk, clears state
func (ui *UI) Reload() error {
	mps, err := db.ReadMps()
	if err != nil {
		ui.log("Could not write MPs to disk", true)
		return err
	}
	ui.state.all = &mps
	ui.state.visible = &mps
	ui.log(fmt.Sprintf("Reloaded %d MPs ", len(*ui.state.visible)), false)

	if v, ok := panelViews[LIST_PANEL]; ok {
		v.text = panel.DrawListMpPanel(ui.gui, &mps)
		if v, err := ui.gui.View(LIST_PANEL); err != gocui.ErrUnknownView {
			v.SetCursor(0, 0)
		}
	}
	ui.selectMp(0)

	// ui.updateView()
	return ui.log("Reloaded <3", false)
}

// toggleSearchingModal opens the SEARCHING_MODAL, then opens LIST_URLS_MODAL with results
// when SEARCHING_MODAL is open, the screen is not interactable. after the LIST_URLS_MODAL is
// opened a URL can be selected
// TODO: support selecting multiple URLs
// TODO: custom help text
func (ui *UI) toggleSearchingModal(g *gocui.Gui) error {
	// panelHeight := strings.Count(content, "\n")
	if ui.currentModal == SEARCHING_MODAL {
		// remove new global key bindings for modal
		// stop modal timer ?
		return ui.closeModal(ui.currentModal)
	}
	var mp data.MP
	if ui.state.currentIndex > len(*ui.state.visible)-1 {
		return nil
	}
	mp = (*ui.state.visible)[ui.state.currentIndex]
	// open thin modal
	v, err := ui.openModal(SEARCHING_MODAL, 10, 1, false)
	if err != nil {
		return err
	}
	// disable cursor while searching
	ui.gui.Cursor = false
	v.Editor = gocui.DefaultEditor
	fmt.Fprintf(v, "Searching for MP: %s", mp.Name())
	// search term, update panel with results
	links, err := ui.searchTerm(mp.GoogleSearchTerm())
	if err != nil || len(links) == 0 {
		if err == nil {
			ui.log("No results were found for MP", true)
		}
		ui.log(fmt.Sprintf("[-] %s", err.Error()), true)
		// close searching modal
		if err := ui.toggleSearchingModal(g); err != nil {
			return err
		}
	}
	ui.closeModal(SEARCHING_MODAL)
	return ui.toggleListUrlsModal(g, links)
}

func (ui *UI) toggleListUrlsModal(g *gocui.Gui, links []searching.Link) error {
	// close modal if already open
	if ui.currentModal == LIST_URLS_MODAL {
		// remove new global key bindings for modal
		// stop modal timer ?
		return ui.closeModal(ui.currentModal)
	}

	newBufferText, newBufferWidth, newBufferHeight, drawErr := panel.DrawListUrlPanel(g, links)
	if drawErr != nil {
		ui.log(fmt.Sprintf("Links that could not be converted to domain,\n%s", drawErr.Error()), true)
	}

	ui.log(fmt.Sprintf("Opening list urls modal width size (%d,%d)", newBufferWidth, newBufferHeight), false)
	v, err := ui.openModal(LIST_URLS_MODAL, newBufferWidth, newBufferHeight, false)
	if err != nil {
		return nil
	}
	v.Wrap = false
	v.SelBgColor = gocui.ColorCyan
	v.SelFgColor = gocui.ColorWhite
	v.Editable = false
	v.Highlight = true
	return ui.writeContent(LIST_URLS_MODAL, newBufferText)
}

// searchTerm
func (ui *UI) searchTerm(term string) ([]searching.Link, error) {
	ui.log(fmt.Sprintf("Searching term <%s>", term), false)
	// go routine to communicate links and err
	links, err := searching.UrlSearcher{}.Search(term)
	if err != nil {
		ui.log(fmt.Sprintf("Error searching <%s> %v", term, err), true)
		return nil, err
	}
	ui.log(fmt.Sprintf("Found %d links", len(links)), false)
	return links, nil
}

// toggleNewDomain toggle the new domain view
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

// toggleHelp toggle the help view on key pressing.
func (ui *UI) toggleHelp(g *gocui.Gui, content string) error {
	panelHeight := strings.Count(content, "\n")
	if ui.currentModal == HELP_PANEL {
		// remove key bindings in original code
		// stop modal timer ?
		return ui.closeModal(ui.currentModal)
	}
	v, err := ui.openModal(HELP_PANEL, 80, panelHeight, true)
	if err != nil {
		return err
	}
	ui.gui.Cursor = false
	v.Editor = nil
	fmt.Fprint(v, content)
	return nil
}

// nextMp chooses the next MP in the LIST_PANEL
func (ui *UI) nextMp(v *gocui.View) error {
	index := wrap(ui.state.currentIndex+1, len(*ui.state.visible))
	if index != ui.state.currentIndex {
		v.MoveCursor(0, 1, true)
	}
	return ui.selectMp(index)
}

// prevMp chooses the previous MP in the LIST_PANEL
func (ui *UI) prevMp(v *gocui.View) error {
	index := wrap(ui.state.currentIndex-1, len(*ui.state.visible))
	if index != ui.state.currentIndex {
		v.MoveCursor(0, -1, true)
	}
	return ui.selectMp(index)
}

// nextDomain chooses the next Domain in the DOMAIN_PANEL
func (ui *UI) nextDomain(v *gocui.View) error {
	index := wrap(ui.state.domainState.index+1, len(*ui.state.domainState.domains))
	if index != ui.state.domainState.index {
		v.MoveCursor(0, 1, true)
	}
	return ui.selectDomain(index)
}

// prevDomain chooses the previous Domain in the DOMAIN_PANEL
func (ui *UI) prevDomain(v *gocui.View) error {
	index := wrap(ui.state.domainState.index-1, len(*ui.state.domainState.domains))
	ui.log(fmt.Sprintf("prevDomain(%d - 1 -> %d)", ui.state.domainState.index, index), false)
	if index != ui.state.currentIndex {
		v.MoveCursor(0, -1, true)
	}
	return ui.selectDomain(index)
}

// selectMp updates the selected MP and updates the listed Domains.
// If an MP is selected the DOMAIN_PANEL state is updated
func (ui *UI) selectMp(newMpIndex int) error {
	ui.log(fmt.Sprintf("selectMp(%d + 1 -> %d)", ui.state.currentIndex, newMpIndex), false)

	if newMpIndex < 0 || newMpIndex >= len(*ui.state.visible) {
		ui.log(fmt.Sprintf("Invalid MP idx %d", newMpIndex), true)
		return nil
	}
	// Store selected MP in state if index has changed
	if newMpIndex != ui.state.currentIndex {
		mp := (*ui.state.visible)[newMpIndex]
		ui.state.currentIndex = newMpIndex
		// update DomainList state
		ui.state.domainState = &DomainState{&mp.Domains, 0}
	}

	var err error
	// update DOMAIN_PANEL, create if missing
	var domainView *gocui.View
	if domainView, err = ui.gui.View(DOMAIN_PANEL); err != nil {
		// create DOMAIN_PANEL
		domainView, err = ui.initPanelView(DOMAIN_PANEL)
		if err != nil {
			return err
		}
	}
	return domainView.SetCursor(0, ui.state.domainState.index)
}

// selectMp updates the selected MP and updates the listed Domains.
// If an MP is selected the DOMAIN_PANEL state is updated
func (ui *UI) selectDomain(newIndex int) error {
	ui.log(fmt.Sprintf("selectDomain(%d -> %d)", ui.state.domainState.index, newIndex), false)
	if newIndex < 0 || newIndex > len(*ui.state.domainState.domains)-1 {
		ui.log(fmt.Sprintf("Invalid Domain idx %d", newIndex), true)
		return nil
	}
	ui.state.domainState.index = newIndex
	// update DOMAIN_PANEL, create if missing
	var domainView *gocui.View
	var err error
	if domainView, err = ui.gui.View(DOMAIN_PANEL); err != nil {
		return err
	}
	return domainView.SetCursor(0, ui.state.domainState.index)
	//ui.log(fmt.Sprintf("[%s] domains tracked: %d", mp.ToString(), len(*ui.state.domainState.domains)), false)
	// return ui.updateView(domainView, ui.printDomains())
}

// checkDomain will update the expiry of the selected domain in the buffer
func (ui *UI) checkDomain() error {
	ui.log("checkDomain()", false)
	_, cy := ui.cursors.Get(DOMAIN_PANEL)
	if cy > len(*ui.state.domainState.domains) {
		ui.log("Please select a domain !", true)
		return nil
	}
	domain := (*ui.state.domainState.domains)[ui.state.domainState.index]
	if msg, err := domain.UpdateExpiry(); err != nil {
		errStr := msg
		if err != nil {
			errStr += "\n" + err.Error()
		}
		ui.log(fmt.Sprintf("Failed check domain <%s>%s", domain.Hostname, errStr), true)
		return err
	}
	ui.log(fmt.Sprintf("Found domain %s expiry %s", domain.Hostname, domain.Expiry), false)
	// update domain
	(*ui.state.domainState.domains)[ui.state.domainState.index] = domain
	//(*ui.state.domainState.domains)[ui.state.domainState.index] = domain
	return nil
}

// addDomainModalTest tests the domain in the ADD_DOMAIN_PANEL
func (ui *UI) addDomainModalTest(addDomainView *gocui.View) error {
	ui.log(fmt.Sprintf("testNewDomain(%s)", addDomainView.Buffer()), false)
	domain := strings.ReplaceAll(addDomainView.Buffer(), "\n", "")
	newDomain := data.Domain{
		Hostname: domain,
		Expiry:   "",
		Expired:  false,
	}
	if errMsg, err := newDomain.UpdateExpiry(); err != nil {
		ui.log(fmt.Sprintf("Cannot add domain <%s> %s", domain, errMsg), true)
		// wipe buffer, let user try again
		ui.ClearView(ADD_DOMAIN_PANEL)
		addDomainView.SetCursor(0, 0)
		return nil
	}
	defer ui.closeModal(ADD_DOMAIN_PANEL)
	return ui.addDomain(domain, ui.state.currentIndex, true)
}

// addDomain adds a domain to a MP. if showDomain is true, the DOMAIN_PANEL is opened and the new domain is
// selected
func (ui *UI) addDomain(domain string, mpIndex int, showDomain bool) error {
	domain = strings.ReplaceAll(domain, "\n", "")
	if mpIndex > len(*ui.state.visible)-1 {
		return ui.log(fmt.Sprintf("No MP with index <%d>", mpIndex), true)
	}
	mp := (*ui.state.visible)[mpIndex]
	newDomain := data.Domain{
		Hostname: domain,
		Expiry:   "",
		Expired:  false,
	}
	// Wipe entered domain if invalid
	if errMsg, err := newDomain.UpdateExpiry(); err != nil {
		ui.log(fmt.Sprintf("Can't check domain <%s> %s", domain, errMsg), true)
	}
	mp.Domains = append(mp.Domains, newDomain)
	(*ui.state.visible)[mpIndex] = mp
	// update MP in visible list
	ui.log(fmt.Sprintf("Added domain <%s> to MP <%s>", newDomain.Hostname, mp.Name()), false)
	ui.log(fmt.Sprintf("After has %d domains", len((*ui.state.visible)[mpIndex].Domains)), false)

	if showDomain {
		ui.state.domainState.domains = &mp.Domains
		ui.selectDomain(len(*ui.state.domainState.domains) - 1)
		// if v, err := ui.gui.View(DOMAIN_PANEL); err != nil {
		// 	// ui.updateView(v, ui.printDomains())
		// }
		return ui.setPanelView(DOMAIN_PANEL)
	}
	return nil
}

// testAllDomains test the domain status of the selected MP
func (ui *UI) testAllDomains() error {
	// var err error
	ui.log("testAllDomains()", false)

	var updatedDomains []data.Domain
	for _, domain := range *ui.state.domainState.domains {
		if errMsg, err := domain.UpdateExpiry(); err != nil {
			ui.log(fmt.Sprintf("Could not update expiry for <%s> %s", domain.Hostname, errMsg), true)
		}
		updatedDomains = append(updatedDomains, domain)
	}

	ui.state.domainState.domains = &updatedDomains

	// Write new Domain to buffer
	// var domainView *gocui.View
	// // update DOMAIN_PANEL, expect it to already exist
	// if domainView, err = ui.gui.View(DOMAIN_PANEL); err != nil {
	// 	return err
	// }
	defer ui.closeModal(ADD_DOMAIN_PANEL)
	return nil
}

func wrap(index, max int) int {
	if index < 0 {
		index = max - 1
	} else if index > max-1 {
		index = 0
	}
	return index
}
