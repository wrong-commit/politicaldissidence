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
      $$\                                   $$\                                      $$\                               
      $$ |                                  \__|                                     \__|                              
 $$$$$$$ | $$$$$$\  $$$$$$\$$$$\   $$$$$$\  $$\ $$$$$$$\          $$$$$$$\ $$$$$$$\  $$\  $$$$$$\   $$$$$$\   $$$$$$\  
$$  __$$ |$$  __$$\ $$  _$$  _$$\  \____$$\ $$ |$$  __$$\        $$  _____|$$  __$$\ $$ |$$  __$$\ $$  __$$\ $$  __$$\ 
$$ /  $$ |$$ /  $$ |$$ / $$ / $$ | $$$$$$$ |$$ |$$ |  $$ |       \$$$$$$\  $$ |  $$ |$$ |$$ /  $$ |$$$$$$$$ |$$ |  \__|
$$ |  $$ |$$ |  $$ |$$ | $$ | $$ |$$  __$$ |$$ |$$ |  $$ |        \____$$\ $$ |  $$ |$$ |$$ |  $$ |$$   ____|$$ |      
\$$$$$$$ |\$$$$$$  |$$ | $$ | $$ |\$$$$$$$ |$$ |$$ |  $$ |       $$$$$$$  |$$ |  $$ |$$ |$$$$$$$  |\$$$$$$$\ $$ |      
 \_______| \______/ \__| \__| \__| \_______|\__|\__|  \__|$$$$$$\\_______/ \__|  \__|\__|$$  ____/  \_______|\__|      
                                                          \______|                       $$ |                          
                                                                                         $$ |                          
                                                                                         \__|                          `

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
