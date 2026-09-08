package ui

/**
 * Expose InitApp to start a new Gui
 * Provide
 */
import (
	"fmt"
	"politicaldissidence/data"
	"politicaldissidence/db"
	"politicaldissidence/refresh"
	"politicaldissidence/ui/panel"
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
	go ui.startBackgroundWhois()
	ui.Loop()
	defer func() { fmt.Println(ui.consoleLog) }()
}

// Save saves written MP and domain details to disk
func (ui *UI) Save() error {
	if err := db.WriteMps(*ui.state.visible); err != nil {
		ui.log("Could not write MPs to disk", true)
		return err
	}
	return ui.log("Saved <3", false)
}

// Load reads and validates MP JSON from disk into UI state (startup).
func (ui *UI) Load() error {
	status := db.ReadMpsValidated()
	_ = ui.logPlain(status.LogMessage())
	if !status.Valid() {
		empty := []data.MP{}
		ui.state.all = &empty
		ui.state.visible = &empty
		return status.Err
	}
	mps := status.MPs
	ui.state.all = &mps
	ui.state.visible = &mps
	ui.log(fmt.Sprintf("Loaded %d MPs ", len(*ui.state.visible)), false)

	if v, ok := panelViews[LIST_PANEL]; ok {
		v.text = panel.DrawListMpPanel(ui.gui, &mps)
	}

	_ = ui.selectMp(0)
	return nil
}

// Reload re-reads and validates MP JSON from disk, replacing in-memory state only when valid.
func (ui *UI) Reload() error {
	status := db.ReadMpsValidated()
	_ = ui.logPlain(status.LogMessage())
	if !status.Valid() {
		return status.Err
	}
	mps := status.MPs
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

	return ui.log("Reloaded <3", false)
}

// nextMp chooses the next MP in the LIST_PANEL
func (ui *UI) nextMp(v *gocui.View) error {
	index := wrap(ui.state.currentIndex+1, len(*ui.state.visible))
	if err := setListCursor(v, index); err != nil {
		return err
	}
	return ui.selectMp(index)
}

// prevMp chooses the previous MP in the LIST_PANEL
func (ui *UI) prevMp(v *gocui.View) error {
	index := wrap(ui.state.currentIndex-1, len(*ui.state.visible))
	if err := setListCursor(v, index); err != nil {
		return err
	}
	return ui.selectMp(index)
}

// nextDomain chooses the next Domain in the DOMAIN_PANEL
func (ui *UI) nextDomain(v *gocui.View) error {
	index := wrap(ui.state.domainState.index+1, len(*ui.state.domainState.domains))
	if err := setListCursor(v, index); err != nil {
		return err
	}
	return ui.selectDomain(index)
}

// prevDomain chooses the previous Domain in the DOMAIN_PANEL
func (ui *UI) prevDomain(v *gocui.View) error {
	index := wrap(ui.state.domainState.index-1, len(*ui.state.domainState.domains))
	// ui.log(fmt.Sprintf("prevDomain(%d - 1 -> %d)", ui.state.domainState.index, index), false)
	if err := setListCursor(v, index); err != nil {
		return err
	}
	return ui.selectDomain(index)
}

// setListCursor places the highlight on absolute line index, scrolling origin as needed.
// Cursor coords in gocui are relative to Origin, so MoveCursor(±1) cannot wrap ends.
func setListCursor(v *gocui.View, index int) error {
	if index < 0 {
		return nil
	}
	ox, oy := v.Origin()
	_, sy := v.Size()
	if sy <= 0 {
		return nil
	}
	if index < oy {
		if err := v.SetOrigin(ox, index); err != nil {
			return err
		}
		return v.SetCursor(0, 0)
	}
	if index >= oy+sy {
		if err := v.SetOrigin(ox, index-sy+1); err != nil {
			return err
		}
		return v.SetCursor(0, sy-1)
	}
	return v.SetCursor(0, index-oy)
}

// selectMp updates the selected MP and updates the listed Domains.
// If an MP is selected the DOMAIN_PANEL state is updated
func (ui *UI) selectMp(newMpIndex int) error {
	// ui.log(fmt.Sprintf("DEBUG selectMp(%d + 1 -> %d)", ui.state.currentIndex, newMpIndex), false)

	if newMpIndex < 0 || newMpIndex >= len(*ui.state.visible) {
		// ui.log(fmt.Sprintf("Invalid MP idx %d", newMpIndex), true)
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
	// ui.log(fmt.Sprintf("DEBUG selectDomain(%d -> %d)", ui.state.domainState.index, newIndex), false)
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

// checkDomain will update the expiry of the selected domain in the buffer.
// Uses the same INFO/DEBUG/INFO log lines as the background WHOIS refresh (no 10-day skip).
func (ui *UI) checkDomain() error {
	if ui.state.domainState == nil || ui.state.domainState.domains == nil {
		return ui.log("Please select a domain !", true)
	}
	idx := ui.state.domainState.index
	if idx < 0 || idx >= len(*ui.state.domainState.domains) {
		return ui.log("Please select a domain !", true)
	}
	mp := (*ui.state.visible)[ui.state.currentIndex]
	one := mp
	one.Domains = (*ui.state.domainState.domains)[idx : idx+1]
	_ = refresh.Run([]data.MP{one}, refresh.Deps{
		Log:   ui.whoisLogger(),
		Force: true,
	})
	return ui.setPanelView(DOMAIN_PANEL)
}

// addDomainModalTest confirms the domain in the ADD_DOMAIN_PANEL and adds it.
// WHOIS runs in the background via addDomain.
func (ui *UI) addDomainModalTest(addDomainView *gocui.View) error {
	ui.log(fmt.Sprintf("testNewDomain(%s)", addDomainView.Buffer()), false)
	domain := strings.ReplaceAll(addDomainView.Buffer(), "\n", "")
	defer ui.closeModal(ADD_DOMAIN_PANEL)
	return ui.addDomain(domain, ui.state.currentIndex, true)
}

// addDomain adds a domain to a MP. if showDomain is true, the DOMAIN_PANEL is opened and the new domain is
// selected. WHOIS for the new domain runs in a background goroutine.
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
	mp.Domains = append(mp.Domains, newDomain)
	(*ui.state.visible)[mpIndex] = mp
	domainIdx := len((*ui.state.visible)[mpIndex].Domains) - 1
	ui.log(fmt.Sprintf("Added domain <%s> to MP <%s>", newDomain.Hostname, mp.Name()), false)
	// ui.log(fmt.Sprintf("After has %d domains", len((*ui.state.visible)[mpIndex].Domains)), false)

	go ui.refreshDomainWhois(mpIndex, domainIdx)

	if showDomain {
		ui.state.domainState.domains = &(*ui.state.visible)[mpIndex].Domains
		ui.selectDomain(domainIdx)
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
