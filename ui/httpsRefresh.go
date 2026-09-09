package ui

import (
	"fmt"
	"time"

	"politicaldissidence/data"
	"politicaldissidence/db"
	"politicaldissidence/httpscheck"
	"politicaldissidence/httpsrefresh"
	"politicaldissidence/ui/panel"
)

func (ui *UI) httpsLogger() httpsrefresh.Logger {
	return httpsrefresh.LogFn{
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

// httpsRefreshDeps builds httpsrefresh.Deps shared by startup scan, u, and add-domain HTTPS.
func (ui *UI) httpsRefreshDeps(force bool, delay time.Duration, save bool) httpsrefresh.Deps {
	deps := httpsrefresh.Deps{
		Log:      ui.httpsLogger(),
		Force:    force,
		Delay:    delay,
		OnLookup: ui.onHttpsLookup,
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

func (ui *UI) onHttpsLookup(mpName string, info httpscheck.Info, err error) {
	_ = mpName
	_ = info
	_ = err
	ui.refreshWhoisPanel()
	ui.refreshDomainPanel()
}

// startBackgroundHttps runs a one-shot background HTTPS refresh after load.
// Honours SKIP_BACKGROUND_HTTPS_LOOKUP=true unless force is true.
// Updates stay in memory until the user saves (Ctrl+S).
func (ui *UI) startBackgroundHttps(force bool) {
	if ui.state.all == nil {
		return
	}
	deps := ui.httpsRefreshDeps(force, httpsrefresh.DefaultLookupDelay, false)
	if _, ok := ui.httpsRunner.TryRun(*ui.state.all, deps); !ok {
		ui.whoisLog("HTTPS refresh already running", false)
	}
	ui.refreshDomainPanel()
	ui.refreshWhoisPanel()
}

// refreshDomainHttps runs a forced HTTPS lookup for one domain off the UI thread.
// mpIndex is an index into state.all (not the filtered visible list).
func (ui *UI) refreshDomainHttps(mpIndex, domainIdx int) {
	if ui.state.all == nil || mpIndex < 0 || mpIndex >= len(*ui.state.all) {
		return
	}
	mp := &(*ui.state.all)[mpIndex]
	if domainIdx < 0 || domainIdx >= len(mp.Domains) {
		return
	}
	one := *mp
	one.Domains = mp.Domains[domainIdx : domainIdx+1]
	_ = httpsrefresh.Run([]data.MP{one}, ui.httpsRefreshDeps(true, 0, false))
	ui.refreshDomainPanel()
	ui.refreshWhoisPanel()
}

// drawSelectedWhois renders WHOIS + HTTPS + DNS for the domain currently selected in Member Domains.
func (ui *UI) drawSelectedWhois() string {
	if !ui.hasDomains() {
		return panel.DrawWhoisPanel("", "", nil, nil, nil, nil)
	}
	idx := ui.state.domainState.index
	domains := *ui.state.domainState.domains
	if idx < 0 || idx >= len(domains) {
		return panel.DrawWhoisPanel("", "", nil, nil, nil, nil)
	}
	mpName := ""
	if mp := ui.mpAt(ui.state.currentIndex); mp != nil {
		mpName = mp.Name()
	}
	d := domains[idx]
	reasons := data.AlertReasons(d, time.Now(), data.AlertSoonWindow)
	return panel.DrawWhoisPanel(d.Hostname, mpName, d.Whois, d.Https, d.DNS, reasons)
}
