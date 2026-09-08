package ui

/**
 * Expose InitApp to start a new Gui
 * Provide
 */
import (
	"fmt"
	"politicaldissidence/data"
	"politicaldissidence/db"
	"politicaldissidence/dnsrefresh"
	"politicaldissidence/httpsrefresh"
	"politicaldissidence/jobs"
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
	go ui.startBackgroundDns()
	go ui.startBackgroundHttps()
	ui.Loop()
	defer func() { fmt.Println(ui.consoleLog) }()
}

// Save saves written MP and domain details to disk
func (ui *UI) Save() error {
	if ui.state.all == nil {
		return ui.log("Could not write MPs to disk: no MPs loaded", true)
	}
	if err := db.WriteMps(*ui.state.all); err != nil {
		return ui.log(fmt.Sprintf("Could not write MPs to disk: %v", err), true)
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
		ui.applyFilter(ui.state.filter)
		return status.Err
	}
	mps := status.MPs
	ui.state.all = &mps
	ui.applyFilter(ui.state.filter)
	ui.log(fmt.Sprintf("Loaded %d MPs ", len(*ui.state.all)), false)

	if v, ok := panelViews[LIST_PANEL]; ok {
		v.text = panel.DrawListMpPanel(ui.gui, ui.state.visible)
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
	ui.applyFilter(ui.state.filter)
	ui.log(fmt.Sprintf("Reloaded %d MPs ", len(mps)), false)

	// Force selectMp to refresh domainState for the reloaded (possibly filtered) list.
	ui.state.currentIndex = -1
	if _, err := ui.initPanelView(LIST_PANEL); err != nil {
		return err
	}
	if v, err := ui.gui.View(LIST_PANEL); err != gocui.ErrUnknownView && v != nil {
		if err := setListCursor(v, 0); err != nil {
			return err
		}
	}
	_ = ui.selectMp(0)

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
	if !ui.hasDomains() {
		return nil
	}
	index := wrap(ui.state.domainState.index+1, len(*ui.state.domainState.domains))
	if err := setListCursor(v, index); err != nil {
		return err
	}
	return ui.selectDomain(index)
}

// prevDomain chooses the previous Domain in the DOMAIN_PANEL
func (ui *UI) prevDomain(v *gocui.View) error {
	if !ui.hasDomains() {
		return nil
	}
	index := wrap(ui.state.domainState.index-1, len(*ui.state.domainState.domains))
	// ui.log(fmt.Sprintf("prevDomain(%d - 1 -> %d)", ui.state.domainState.index, index), false)
	if err := setListCursor(v, index); err != nil {
		return err
	}
	return ui.selectDomain(index)
}

func (ui *UI) hasDomains() bool {
	return ui.state.domainState != nil &&
		ui.state.domainState.domains != nil &&
		len(*ui.state.domainState.domains) > 0
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

	mp := ui.mpAt(newMpIndex)
	if mp == nil {
		// ui.log(fmt.Sprintf("Invalid MP idx %d", newMpIndex), true)
		return nil
	}
	// Store selected MP in state if index has changed (or domainState was never set)
	if newMpIndex != ui.state.currentIndex || ui.state.domainState == nil {
		ui.state.currentIndex = newMpIndex
		// Point at Domains on the canonical all slice, not a filtered copy
		ui.state.domainState = &DomainState{&mp.Domains, 0}
	}

	domainView, err := ui.initPanelView(DOMAIN_PANEL)
	if err != nil {
		return err
	}
	if err := setListCursor(domainView, ui.state.domainState.index); err != nil {
		return err
	}
	_, _ = ui.initPanelView(WHOIS_PANEL)
	return nil
}

// selectMp updates the selected MP and updates the listed Domains.
// If an MP is selected the DOMAIN_PANEL state is updated
func (ui *UI) selectDomain(newIndex int) error {
	// ui.log(fmt.Sprintf("DEBUG selectDomain(%d -> %d)", ui.state.domainState.index, newIndex), false)
	if !ui.hasDomains() {
		return nil
	}
	if newIndex < 0 || newIndex > len(*ui.state.domainState.domains)-1 {
		ui.log(fmt.Sprintf("Invalid Domain idx %d", newIndex), true)
		return nil
	}
	ui.state.domainState.index = newIndex
	domainView, err := ui.initPanelView(DOMAIN_PANEL)
	if err != nil {
		return err
	}
	if err := setListCursor(domainView, ui.state.domainState.index); err != nil {
		return err
	}
	_, _ = ui.initPanelView(WHOIS_PANEL)
	return nil
}

// checkDomain will update WHOIS expiry, DNS, and HTTPS for the selected domain in the buffer.
// Uses the same INFO/DEBUG/INFO log lines as the background refresh jobs (no 10-day skip).
func (ui *UI) checkDomain() error {
	if ui.state.domainState == nil || ui.state.domainState.domains == nil {
		return ui.log("Please select a domain !", true)
	}
	idx := ui.state.domainState.index
	if idx < 0 || idx >= len(*ui.state.domainState.domains) {
		return ui.log("Please select a domain !", true)
	}
	mp := ui.mpAt(ui.state.currentIndex)
	if mp == nil {
		return ui.log("Please select a domain !", true)
	}
	one := *mp
	one.Domains = (*ui.state.domainState.domains)[idx : idx+1]
	_ = refresh.Run([]data.MP{one}, ui.whoisRefreshDeps(true, 0, false))
	_ = dnsrefresh.Run([]data.MP{one}, ui.dnsRefreshDeps(true, 0, false))
	_ = httpsrefresh.Run([]data.MP{one}, ui.httpsRefreshDeps(true, 0, false))
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
// selected. Registered domainAddedJobs (WHOIS, calc demo, …) run in background goroutines.
func (ui *UI) addDomain(domain string, mpIndex int, showDomain bool) error {
	domain = strings.ReplaceAll(domain, "\n", "")
	mp := ui.mpAt(mpIndex)
	if mp == nil {
		return ui.log(fmt.Sprintf("No MP with index <%d>", mpIndex), true)
	}
	// Jobs run async after applyFilter; pass an index into all, not the visible row.
	allIdx := mpIndex
	if ui.state.visibleIdx != nil {
		allIdx = ui.state.visibleIdx[mpIndex]
	}
	want := strings.ToLower(strings.TrimSpace(domain))
	for _, existing := range mp.Domains {
		if strings.ToLower(strings.TrimSpace(existing.Hostname)) == want {
			return ui.log(fmt.Sprintf("Domain <%s> already exists for MP <%s>", domain, mp.Name()), true)
		}
	}
	newDomain := data.Domain{
		Hostname: domain,
		Expiry:   "",
		Expired:  false,
	}
	mp.Domains = append(mp.Domains, newDomain)
	domainIdx := len(mp.Domains) - 1
	ui.log(fmt.Sprintf("Added domain <%s> to MP <%s>", newDomain.Hostname, mp.Name()), false)

	jobs.Kick(jobs.Context{
		MPIndex:   allIdx,
		DomainIdx: domainIdx,
		Hostname:  newDomain.Hostname,
		Log:       ui.whoisLog,
	}, ui.domainAddedJobs...)

	// Keep filtered display list in sync (and drop MPs that no longer match).
	ui.applyFilter(ui.state.filter)

	if showDomain {
		// Find this MP again in the (possibly rebuilt) visible list.
		visIdx := ui.visibleIndexOfAll(allIdx)
		if visIdx < 0 {
			ui.state.currentIndex = -1
			_ = ui.selectMp(0)
			return ui.setPanelView(LIST_PANEL)
		}
		ui.state.currentIndex = visIdx
		ui.state.domainState = &DomainState{&(*ui.state.all)[allIdx].Domains, domainIdx}
		ui.selectDomain(domainIdx)
		return ui.setPanelView(DOMAIN_PANEL)
	}
	return nil
}

// visibleIndexOfAll returns the LIST_PANEL row for an index into all, or -1 if filtered out.
func (ui *UI) visibleIndexOfAll(allIdx int) int {
	if ui.state.all == nil || allIdx < 0 || allIdx >= len(*ui.state.all) {
		return -1
	}
	if ui.state.visibleIdx == nil {
		return allIdx
	}
	for i, j := range ui.state.visibleIdx {
		if j == allIdx {
			return i
		}
	}
	return -1
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
	if max <= 0 {
		return -1
	}
	if index < 0 {
		index = max - 1
	} else if index > max-1 {
		index = 0
	}
	return index
}
