package panel

import (
	"fmt"
	"politicaldissidence/data"
	"time"

	"github.com/jroimartin/gocui"
)

// decorate changes the color of a string
func decorate(s string, color string) string {
	switch color {
	// case "green":
	// 	s = "\x1b[0;32m" + s
	// case "red":
	// 	s = "\x1b[0;31m" + s
	default:
		return s
	}
	// return s + "\x1b[0m"
}

const lastCheckedLayout = "06-01-02"

func formatLastChecked(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	return t.Format(lastCheckedLayout)
}

func AlertMarker(alert bool) string {
	if alert {
		return "alert: true"
	}
	return ""
}

func DrawListDomainPanel(g *gocui.Gui, domains *[]data.Domain) string {
	var sb string
	for i, domain := range *domains {
		expiry := domain.Expiry
		if expiry == "" {
			expiry = "<?>"
		}
		if domain.Expired {
			expiry += "[!]"
		}
		if domain.Alert {
			expiry += "[x]"
		}
		host := domain.Hostname
		if domain.Alert {
			host = "[!] " + host
		}
		alert := AlertMarker(domain.Alert)
		if alert != "" {
			sb += fmt.Sprintf("\t%d. %s %s  checked %s  %s\n", i+1, host, expiry, formatLastChecked(domain.LastChecked), alert)
		} else {
			sb += fmt.Sprintf("\t%d. %s %s  checked %s\n", i+1, host, expiry, formatLastChecked(domain.LastChecked))
		}
	}
	return sb
}
