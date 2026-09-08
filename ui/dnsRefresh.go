package ui

import (
	"fmt"
	"time"

	"politicaldissidence/data"
	"politicaldissidence/db"
	"politicaldissidence/dnscheck"
	"politicaldissidence/dnsrefresh"
)

func (ui *UI) dnsLogger() dnsrefresh.Logger {
	return dnsrefresh.LogFn{
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

// dnsRefreshDeps builds dnsrefresh.Deps shared by startup scan, u, and add-domain DNS.
func (ui *UI) dnsRefreshDeps(force bool, delay time.Duration, save bool) dnsrefresh.Deps {
	deps := dnsrefresh.Deps{
		Log:      ui.dnsLogger(),
		Force:    force,
		Delay:    delay,
		OnLookup: ui.onDnsLookup,
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

func (ui *UI) onDnsLookup(mpName string, info dnscheck.Info, err error) {
	_ = mpName
	_ = info
	_ = err
	ui.refreshWhoisPanel()
	ui.refreshDomainPanel()
}

// startBackgroundDns runs a one-shot background DNS refresh after load.
// Honours SKIP_BACKGROUND_DNS_LOOKUP=true unless force is true.
func (ui *UI) startBackgroundDns(force bool) {
	if ui.state.all == nil {
		return
	}
	deps := ui.dnsRefreshDeps(force, dnsrefresh.DefaultLookupDelay, true)
	if _, ok := ui.dnsRunner.TryRun(*ui.state.all, deps); !ok {
		ui.whoisLog("DNS refresh already running", false)
	}
	ui.refreshDomainPanel()
	ui.refreshWhoisPanel()
}

// refreshDomainDns runs a forced DNS lookup for one domain off the UI thread.
// mpIndex is an index into state.all (not the filtered visible list).
func (ui *UI) refreshDomainDns(mpIndex, domainIdx int) {
	if ui.state.all == nil || mpIndex < 0 || mpIndex >= len(*ui.state.all) {
		return
	}
	mp := &(*ui.state.all)[mpIndex]
	if domainIdx < 0 || domainIdx >= len(mp.Domains) {
		return
	}
	one := *mp
	one.Domains = mp.Domains[domainIdx : domainIdx+1]
	_ = dnsrefresh.Run([]data.MP{one}, ui.dnsRefreshDeps(true, 0, false))
	ui.refreshDomainPanel()
	ui.refreshWhoisPanel()
}
