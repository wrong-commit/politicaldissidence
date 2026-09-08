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
			ui.whoisLog(msg)
		},
		OnDebug: func(msg string) {
			ui.whoisLog(msg)
		},
		OnError: func(msg string) {
			ui.whoisLog(msg)
		},
	}
}

func (ui *UI) whoisLog(msg string) {
	if ui.gui == nil || !ui.started {
		_ = ui.logPlain(msg)
		return
	}
	ui.gui.Update(func(g *gocui.Gui) error {
		return ui.logPlain(msg)
	})
}

// startBackgroundWhois runs a one-shot background WHOIS refresh after load.
// Honours SKIP_BACKGROUND_WHOIS_LOOKUP=true.
func (ui *UI) startBackgroundWhois() {
	if ui.state.all == nil {
		return
	}
	deps := refresh.Deps{
		Log: ui.whoisLogger(),
		Save: func(mps []data.MP) error {
			return db.WriteMps(mps)
		},
	}
	_, _ = ui.whoisRunner.TryRun(*ui.state.all, deps)
	if ui.gui != nil {
		ui.gui.Update(func(g *gocui.Gui) error {
			if ui.state.domainState != nil && ui.state.domainState.domains != nil {
				return ui.setPanelView(DOMAIN_PANEL)
			}
			return nil
		})
	}
}
