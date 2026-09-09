package ui

import (
	"strings"
	"testing"
	"time"
)

func TestPeriodicDomainChecksEnabled(t *testing.T) {
	t.Setenv(EnvSkipPeriodicDomainChecks, "")
	if !PeriodicDomainChecksEnabled() {
		t.Fatal("unset env should enable ticker")
	}
	t.Setenv(EnvSkipPeriodicDomainChecks, "false")
	if !PeriodicDomainChecksEnabled() {
		t.Fatal("false should enable ticker")
	}
	t.Setenv(EnvSkipPeriodicDomainChecks, "true")
	if PeriodicDomainChecksEnabled() {
		t.Fatal("true should disable ticker")
	}
	t.Setenv(EnvSkipPeriodicDomainChecks, "TRUE")
	if PeriodicDomainChecksEnabled() {
		t.Fatal("TRUE should disable ticker")
	}
}

func TestFormatPeriodicDomainChecksTickerSkipped(t *testing.T) {
	got := FormatPeriodicDomainChecksTickerSkipped()
	if !strings.Contains(got, EnvSkipPeriodicDomainChecks) {
		t.Fatalf("expected env name in %q", got)
	}
	if !strings.HasPrefix(got, "INFO ") {
		t.Fatalf("expected INFO prefix, got %q", got)
	}
}

func TestDefaultPeriodicDomainCheckInterval(t *testing.T) {
	if DefaultPeriodicDomainCheckInterval != 30*time.Minute {
		t.Fatalf("got %v want 30m", DefaultPeriodicDomainCheckInterval)
	}
}
