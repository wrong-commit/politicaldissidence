package panel

import (
	"fmt"
	"politicaldissidence/data"

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
		sb += fmt.Sprintf("\t%d. %s %s\n", i+1, domain.Hostname, expiry)
	}
	return sb
}
