package ui

import (
	"politicaldissidence/data"
	"politicaldissidence/db"
	"politicaldissidence/refresh"

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

// startBackgroundWhois runs a one-shot background WHOIS refresh after load.
// Honours SKIP_BACKGROUND_WHOIS_LOOKUP=true.
func (ui *UI) startBackgroundWhois() {
	if ui.state.all == nil {
		return
	}
	deps := refresh.Deps{
		Log:   ui.whoisLogger(),
		Delay: refresh.DefaultLookupDelay,
		Save: func(mps []data.MP) error {
			return db.WriteMps(mps)
		},
	}
	_, _ = ui.whoisRunner.TryRun(*ui.state.all, deps)
	ui.refreshDomainPanel()
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
	_ = refresh.Run([]data.MP{one}, refresh.Deps{
		Log:   ui.whoisLogger(),
		Force: true,
	})
	ui.refreshDomainPanel()
}

func (ui *UI) refreshDomainPanel() {
	if ui.gui == nil {
		return
	}
	ui.gui.Update(func(g *gocui.Gui) error {
		if ui.state.domainState != nil && ui.state.domainState.domains != nil {
			return ui.setPanelView(DOMAIN_PANEL)
		}
		return nil
	})
}
