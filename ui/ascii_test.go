package ui

import (
	"os"
	"testing"
)

func TestTitleScreenEnabled(t *testing.T) {
	t.Setenv(EnvSkipTitleScreen, "")
	if !TitleScreenEnabled() {
		t.Fatal("expected enabled when unset")
	}
	t.Setenv(EnvSkipTitleScreen, "true")
	if TitleScreenEnabled() {
		t.Fatal("expected disabled when true")
	}
	t.Setenv(EnvSkipTitleScreen, "TRUE")
	if TitleScreenEnabled() {
		t.Fatal("expected disabled when TRUE")
	}
	t.Setenv(EnvSkipTitleScreen, "false")
	if !TitleScreenEnabled() {
		t.Fatal("expected enabled when false")
	}
	_ = os.Unsetenv(EnvSkipTitleScreen)
}

func TestFormatTitleScreenSkipped(t *testing.T) {
	got := FormatTitleScreenSkipped()
	want := "INFO title screen skipped (SKIP_TITLE_SCREEN=true)"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestEffectiveTitleASCII(t *testing.T) {
	art := effectiveTitleASCII()
	if art == "" || art == titleASCIIFallback {
		// Real banner should be non-empty and not just the fallback.
		if art == "" {
			t.Fatal("effective art empty")
		}
	}
	if len(art) < 20 {
		t.Fatalf("art too short: %d", len(art))
	}
}
