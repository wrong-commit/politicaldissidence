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
	"unicode/utf8"

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
	ui.loadCsvRefreshConfig()
	ui.armCsvRefreshTicker() // first fire after one interval; no run on startup
	go ui.startBackgroundWhois(false)
	go ui.startBackgroundDns(false)
	go ui.startBackgroundHttps(false)
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
	ui.log(fmt.Sprintf(" Loaded %d MPs ", len(*ui.state.all)), false)

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

// setListCursor places the highlight on absolute buffer-line index, scrolling origin as needed.
// Cursor coords in gocui are relative to Origin, so MoveCursor(±1) cannot wrap ends.
// When v.Wrap is set, Origin Y indexes visual (wrapped) rows, not buffer lines — map accordingly.
func setListCursor(v *gocui.View, index int) error {
	if index < 0 {
		return nil
	}
	ox, oy := v.Origin()
	sx, sy := v.Size()
	if sy <= 0 {
		return nil
	}

	visStart := index
	visEnd := index
	if v.Wrap {
		starts := visualLineStarts(v.BufferLines(), sx)
		if len(starts) < 2 {
			return v.SetCursor(0, 0)
		}
		if index > len(starts)-2 {
			index = len(starts) - 2
		}
		visStart = starts[index]
		visEnd = starts[index+1] - 1
	}

	newOy := oy
	if visStart < oy {
		newOy = visStart
	} else if visEnd >= oy+sy {
		newOy = visEnd - sy + 1
		if newOy > visStart {
			newOy = visStart
		}
		if newOy < 0 {
			newOy = 0
		}
	}
	if newOy != oy {
		if err := v.SetOrigin(ox, newOy); err != nil {
			return err
		}
	}
	cy := visStart - newOy
	if cy < 0 {
		cy = 0
	}
	if cy >= sy {
		cy = sy - 1
	}
	return v.SetCursor(0, cy)
}

// wrappedLineCount matches gocui's Wrap layout: lines shorter than width stay one row;
// otherwise ceil(len/width) plus the empty trailing chunk when len is an exact multiple.
func wrappedLineCount(cellLen, width int) int {
	if width <= 0 {
		width = 1
	}
	if cellLen < width {
		return 1
	}
	return cellLen/width + 1
}

// visualLineStarts returns the first visual-row index for each buffer line, matching gocui Wrap.
// The final element is the total visual row count (one past the last buffer line).
func visualLineStarts(bufferLines []string, width int) []int {
	starts := make([]int, len(bufferLines)+1)
	vis := 0
	for i, line := range bufferLines {
		starts[i] = vis
		vis += wrappedLineCount(utf8.RuneCountInString(line), width)
	}
	starts[len(bufferLines)] = vis
	return starts
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

// rerunBackgroundChecks kicks WHOIS, DNS, and HTTPS background scans for all domains.
// When force is false, fresh lastChecked / checkedAt values are skipped (same as startup).
// When force is true, every domain is rechecked and SKIP_BACKGROUND_* env skips are ignored.
func (ui *UI) rerunBackgroundChecks(force bool) error {
	if ui.state.all == nil {
		return ui.log("Could not start domain checks: no MPs loaded", true)
	}
	if force {
		_ = ui.log("Force-checking all domains (WHOIS/DNS/HTTPS)", false)
	} else {
		_ = ui.log("Rechecking domains (WHOIS/DNS/HTTPS), skipping fresh lastChecked", false)
	}
	go ui.startBackgroundWhois(force)
	go ui.startBackgroundDns(force)
	go ui.startBackgroundHttps(force)
	return nil
}

// addDomainModalTest confirms the domain in the ADD_DOMAIN_PANEL and adds it.
// WHOIS runs in the background via addDomain.
func (ui *UI) addDomainModalTest(addDomainView *gocui.View) error {
	ui.log(fmt.Sprintf("testNewDomain(%s)", addDomainView.Buffer()), false)
	domain := strings.ReplaceAll(addDomainView.Buffer(), "\n", "")
	defer ui.closeModal(ADD_DOMAIN_PANEL)
	return ui.addDomain(domain, ui.state.currentIndex, true)
}

// removeDomainAt deletes domains[idx] and returns the new slice plus a selection index
// clamped into the remaining list (0 when empty).
func removeDomainAt(domains []data.Domain, idx int) (out []data.Domain, newIdx int, removed data.Domain, ok bool) {
	if idx < 0 || idx >= len(domains) {
		return domains, idx, data.Domain{}, false
	}
	removed = domains[idx]
	// Full slice expression avoids aliasing capacity into the tail.
	out = append(domains[:idx:idx], domains[idx+1:]...)
	if len(out) == 0 {
		return out, 0, removed, true
	}
	newIdx = idx
	if newIdx >= len(out) {
		newIdx = len(out) - 1
	}
	return out, newIdx, removed, true
}

// removeDomain deletes the currently selected domain from the current MP.
func (ui *UI) removeDomain() error {
	if ui.state.domainState == nil || ui.state.domainState.domains == nil {
		return ui.log("Please select a domain !", true)
	}
	idx := ui.state.domainState.index
	mp := ui.mpAt(ui.state.currentIndex)
	if mp == nil {
		return ui.log("Please select a domain !", true)
	}
	allIdx := ui.state.currentIndex
	if ui.state.visibleIdx != nil {
		allIdx = ui.state.visibleIdx[ui.state.currentIndex]
	}

	newDomains, newIdx, removed, ok := removeDomainAt(mp.Domains, idx)
	if !ok {
		return ui.log("Please select a domain !", true)
	}
	mp.Domains = newDomains
	ui.log(fmt.Sprintf("Removed domain <%s> from MP <%s>", removed.Hostname, mp.Name()), false)

	// Keep filtered display list in sync (and drop MPs that no longer match).
	ui.applyFilter(ui.state.filter)

	visIdx := ui.visibleIndexOfAll(allIdx)
	if visIdx < 0 {
		ui.state.currentIndex = -1
		_ = ui.selectMp(0)
		return ui.setPanelView(LIST_PANEL)
	}
	ui.state.currentIndex = visIdx
	ui.state.domainState = &DomainState{&(*ui.state.all)[allIdx].Domains, newIdx}
	if len(newDomains) > 0 {
		_ = ui.selectDomain(newIdx)
	} else {
		_, _ = ui.initPanelView(DOMAIN_PANEL)
		_, _ = ui.initPanelView(WHOIS_PANEL)
	}
	return ui.setPanelView(DOMAIN_PANEL)
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
