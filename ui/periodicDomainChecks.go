package ui

import (
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	// EnvSkipPeriodicDomainChecks disables the 30-minute WHOIS/DNS/HTTPS/registrar recheck ticker
	// when set to "true" (case-insensitive). Startup checks and manual `u` are unaffected.
	EnvSkipPeriodicDomainChecks = "SKIP_PERIODIC_DOMAIN_CHECKS"

	// DefaultPeriodicDomainCheckInterval is how often the TUI re-runs due domain checks.
	DefaultPeriodicDomainCheckInterval = 30 * time.Minute
)

// PeriodicDomainChecksEnabled reports whether the periodic WHOIS/DNS/HTTPS ticker should arm.
// SKIP_PERIODIC_DOMAIN_CHECKS=true disables it; startup scans and manual `u` are unaffected.
func PeriodicDomainChecksEnabled() bool {
	v := strings.TrimSpace(os.Getenv(EnvSkipPeriodicDomainChecks))
	return !strings.EqualFold(v, "true")
}

func FormatPeriodicDomainChecksTickerSkipped() string {
	return fmt.Sprintf("INFO periodic domain checks ticker skipped (%s=true)", EnvSkipPeriodicDomainChecks)
}

// armPeriodicDomainChecksTicker starts a ticker that re-runs the same background
// WHOIS / DNS / HTTPS scans as startup (force=false → only domains not checked recently).
// First fire is after one full interval; startup already ran once in InitApp.
func (ui *UI) armPeriodicDomainChecksTicker() {
	if !PeriodicDomainChecksEnabled() {
		_ = ui.logPlain(FormatPeriodicDomainChecksTickerSkipped())
		return
	}
	interval := DefaultPeriodicDomainCheckInterval
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			go ui.startBackgroundWhois(false)
			go ui.startBackgroundDns(false)
			go ui.startBackgroundHttps(false)
			go ui.startBackgroundRegistrar(false)
		}
	}()
}
