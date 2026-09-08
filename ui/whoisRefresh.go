package ui

import (
	"time"

	"politicaldissidence/data"
	"politicaldissidence/db"
	"politicaldissidence/refresh"
	"politicaldissidence/ui/panel"
	"politicaldissidence/whois"

	"github.com/jroimartin/gocui"
)

func (ui *UI) whoisLogger() refresh.Logger {
	return refresh.LogFn{
		OnInfo: func(msg string) {
			ui.whoisLog(msg, false)
		},
		OnDebug: func(msg string) {
			ui.whoisLog(msg, false)
		},
		OnError: func(msg string) {
			ui.whoisLog(msg, true)
		},
	}
}

func (ui *UI) whoisLog(msg string, isError bool) {
	if ui.gui == nil || !ui.started {
		_ = ui.log(msg, isError)
		return
	}
	ui.gui.Update(func(g *gocui.Gui) error {
		return ui.log(msg, isError)
	})
}

// whoisRefreshDeps builds refresh.Deps shared by startup scan, Ctrl+U, and add-domain WHOIS.
func (ui *UI) whoisRefreshDeps(force bool, delay time.Duration, save bool) refresh.Deps {
	deps := refresh.Deps{
		Log:      ui.whoisLogger(),
		Force:    force,
		Delay:    delay,
		OnLookup: ui.onWhoisLookup,
	}
	if save {
		deps.Save = func(mps []data.MP) error {
			return db.WriteMps(mps)
		}
	}
	return deps
}

// onWhoisLookup refreshes the WHOIS panel after a lookup is stored on the domain.
// The panel always reflects the currently selected Member Domains row.
func (ui *UI) onWhoisLookup(mpName string, info whois.Info, err error) {
	_ = mpName
	_ = info
	_ = err
	ui.refreshWhoisPanel()
}

// startBackgroundWhois runs a one-shot background WHOIS refresh after load.
// Honours SKIP_BACKGROUND_WHOIS_LOOKUP=true.
func (ui *UI) startBackgroundWhois() {
	if ui.state.all == nil {
		return
	}
	deps := ui.whoisRefreshDeps(false, refresh.DefaultLookupDelay, true)
	_, _ = ui.whoisRunner.TryRun(*ui.state.all, deps)
	ui.refreshDomainPanel()
	ui.refreshWhoisPanel()
}

// refreshDomainWhois runs a forced WHOIS lookup for one domain off the UI thread,
// then redraws the domain panel. Used when a domain is newly added.
func (ui *UI) refreshDomainWhois(mpIndex, domainIdx int) {
	if ui.state.visible == nil || mpIndex < 0 || mpIndex >= len(*ui.state.visible) {
		return
	}
	if domainIdx < 0 || domainIdx >= len((*ui.state.visible)[mpIndex].Domains) {
		return
	}
	mp := (*ui.state.visible)[mpIndex]
	one := mp
	one.Domains = (*ui.state.visible)[mpIndex].Domains[domainIdx : domainIdx+1]
	_ = refresh.Run([]data.MP{one}, ui.whoisRefreshDeps(true, 0, false))
	ui.refreshDomainPanel()
	ui.refreshWhoisPanel()
}

func (ui *UI) refreshDomainPanel() {
	if ui.gui == nil {
		return
	}
	ui.gui.Update(func(g *gocui.Gui) error {
		if ui.state.domainState == nil || ui.state.domainState.domains == nil {
			return nil
		}
		// Redraw in place — do not steal focus from LIST_PANEL (or elsewhere).
		if _, err := ui.initPanelView(DOMAIN_PANEL); err != nil {
			return err
		}
		v, err := g.View(DOMAIN_PANEL)
		if err != nil {
			return err
		}
		return setListCursor(v, ui.state.domainState.index)
	})
}

func (ui *UI) refreshWhoisPanel() {
	if ui.gui == nil {
		return
	}
	ui.gui.Update(func(g *gocui.Gui) error {
		_, err := ui.initPanelView(WHOIS_PANEL)
		return err
	})
}

// drawSelectedWhois renders WHOIS for the domain currently selected in Member Domains.
func (ui *UI) drawSelectedWhois() string {
	if !ui.hasDomains() {
		return panel.DrawWhoisPanel("", "", nil)
	}
	idx := ui.state.domainState.index
	domains := *ui.state.domainState.domains
	if idx < 0 || idx >= len(domains) {
		return panel.DrawWhoisPanel("", "", nil)
	}
	mpName := ""
	if ui.state.visible != nil && ui.state.currentIndex >= 0 && ui.state.currentIndex < len(*ui.state.visible) {
		mpName = (*ui.state.visible)[ui.state.currentIndex].Name()
	}
	d := domains[idx]
	return panel.DrawWhoisPanel(d.Hostname, mpName, d.Whois)
}
