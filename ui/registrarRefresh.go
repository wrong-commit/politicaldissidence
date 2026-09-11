package ui

import (
	"fmt"
	"time"

	"politicaldissidence/data"
	"politicaldissidence/db"
	"politicaldissidence/registrarcheck"
	"politicaldissidence/registrarrefresh"
)

func (ui *UI) registrarLogger() registrarrefresh.Logger {
	return registrarrefresh.LogFn{
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

func (ui *UI) registrarRefreshDeps(force bool, delay time.Duration, save bool) registrarrefresh.Deps {
	deps := registrarrefresh.Deps{
		Log:      ui.registrarLogger(),
		Force:    force,
		Delay:    delay,
		OnLookup: ui.onRegistrarLookup,
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

func (ui *UI) onRegistrarLookup(mpName string, info registrarcheck.Info, err error) {
	_ = mpName
	_ = info
	_ = err
	ui.refreshWhoisPanel()
	ui.refreshDomainPanel()
}

func (ui *UI) startBackgroundRegistrar(force bool) {
	if ui.state.all == nil {
		return
	}
	deps := ui.registrarRefreshDeps(force, registrarrefresh.DefaultLookupDelay, false)
	if _, ok := ui.registrarRunner.TryRun(*ui.state.all, deps); !ok {
		ui.whoisLog("registrar refresh already running", false)
	}
	ui.refreshDomainPanel()
	ui.refreshWhoisPanel()
}

func (ui *UI) refreshDomainRegistrar(mpIndex, domainIdx int) {
	if ui.state.all == nil || mpIndex < 0 || mpIndex >= len(*ui.state.all) {
		return
	}
	mp := &(*ui.state.all)[mpIndex]
	if domainIdx < 0 || domainIdx >= len(mp.Domains) {
		return
	}
	one := *mp
	one.Domains = mp.Domains[domainIdx : domainIdx+1]
	_ = registrarrefresh.Run([]data.MP{one}, ui.registrarRefreshDeps(true, 0, false))
	ui.refreshDomainPanel()
	ui.refreshWhoisPanel()
}
