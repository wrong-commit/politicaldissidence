package ui

import (
	"fmt"
	"os"
	"strings"
)

const (
	// EnvSkipTitleScreen disables the startup ASCII title panel when set to "true"
	// (case-insensitive). Load / tickers / background jobs are unaffected.
	EnvSkipTitleScreen = "SKIP_TITLE_SCREEN"

	titleASCIIFallback = "Political Dissidence"
)

// titleASCII is the startup banner art (source of truth; not loaded from disk).
const titleASCII = `
              .__  .__  __  .__              .__        .___.__              .__    .___                          
______   ____ |  | |__|/  |_|__| ____ _____  |  |     __| _/|__| ______ _____|__| __| _/____   ____   ____  ____  
\____ \ /  _ \|  | |  \   __\  |/ ___\\__  \ |  |    / __ | |  |/  ___//  ___/  |/ __ |/ __ \ /    \_/ ___\/ __ \ 
|  |_> >  <_> )  |_|  ||  | |  \  \___ / __ \|  |__ / /_/ | |  |\___ \ \___ \|  / /_/ \  ___/|   |  \  \__\  ___/ 
|   __/ \____/|____/__||__| |__|\___  >____  /____/ \____ | |__/____  >____  >__\____ |\___  >___|  /\___  >___  >
|__|                                \/     \/            \/         \/     \/        \/    \/     \/     \/    \/ 
`

// TitleScreenEnabled reports whether the startup title panel should show.
// SKIP_TITLE_SCREEN=true disables it.
func TitleScreenEnabled() bool {
	v := strings.TrimSpace(os.Getenv(EnvSkipTitleScreen))
	return !strings.EqualFold(v, "true")
}

// FormatTitleScreenSkipped is the INFO line when the title is skipped via env.
func FormatTitleScreenSkipped() string {
	return fmt.Sprintf("INFO title screen skipped (%s=true)", EnvSkipTitleScreen)
}

// effectiveTitleASCII returns the banner art, or a short fallback when empty.
func effectiveTitleASCII() string {
	if strings.TrimSpace(titleASCII) == "" {
		return titleASCIIFallback
	}
	s := titleASCII
	if strings.HasPrefix(s, "\n") {
		s = s[1:]
	}
	return strings.TrimRight(s, "\n")
}
