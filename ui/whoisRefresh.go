package ui

import (
	"fmt"
	"time"

	"politicaldissidence/data"
	"politicaldissidence/db"
	"politicaldissidence/refresh"
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
			if err := db.WriteMps(mps); err != nil {
				ui.whoisLog(fmt.Sprintf("Could not write MPs to disk: %v", err), true)
				return err
			}
			return nil
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
// Honours SKIP_BACKGROUND_WHOIS_LOOKUP=true unless force is true.
// Updates stay in memory until the user saves (Ctrl+S).
func (ui *UI) startBackgroundWhois(force bool) {
	if ui.state.all == nil {
		return
	}
	deps := ui.whoisRefreshDeps(force, refresh.DefaultLookupDelay, false)
	if _, ok := ui.whoisRunner.TryRun(*ui.state.all, deps); !ok {
		ui.whoisLog("WHOIS refresh already running", false)
	}
	ui.refreshDomainPanel()
	ui.refreshWhoisPanel()
}

// refreshDomainWhois runs a forced WHOIS lookup for one domain off the UI thread,
// then redraws the domain panel. Used when a domain is newly added.
// mpIndex is an index into state.all (not the filtered visible list).
func (ui *UI) refreshDomainWhois(mpIndex, domainIdx int) {
	if ui.state.all == nil || mpIndex < 0 || mpIndex >= len(*ui.state.all) {
		return
	}
	mp := &(*ui.state.all)[mpIndex]
	if domainIdx < 0 || domainIdx >= len(mp.Domains) {
		return
	}
	one := *mp
	one.Domains = mp.Domains[domainIdx : domainIdx+1]
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
