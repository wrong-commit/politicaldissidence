package panel

import (
	"fmt"
	"politicaldissidence/data"
	"strings"
)

const whoisCheckedLayout = "06-01-02 15:04"

// DrawWhoisPanel renders the WHOIS information panel for a domain's latest record.
// checkedAt is shown at the top when present.
func DrawWhoisPanel(hostname, mpName string, w *data.WhoisRecord) string {
	if w == nil {
		return "No WHOIS lookup yet"
	}

	var b strings.Builder
	if !w.CheckedAt.IsZero() {
		fmt.Fprintf(&b, "Checked: %s\n", w.CheckedAt.Format(whoisCheckedLayout))
	}

	host := hostname
	if host == "" {
		host = "(unknown)"
	}
	fmt.Fprintf(&b, "%s\n", host)

	if w.Error != "" {
		fmt.Fprintf(&b, "Error: %s\n", w.Error)
		if mpName != "" {
			fmt.Fprintf(&b, "MP: %s\n", mpName)
		}
		return strings.TrimSuffix(b.String(), "\n")
	}

	if mpName != "" {
		fmt.Fprintf(&b, "MP: %s\n", mpName)
	}
	b.WriteByte('\n')

	if len(w.Status) > 0 {
		fmt.Fprintf(&b, "Status: %s\n", strings.Join(w.Status, ", "))
	}
	if w.Created != "" {
		fmt.Fprintf(&b, "Created: %s\n", w.Created)
	}
	if w.Updated != "" {
		fmt.Fprintf(&b, "Updated: %s\n", w.Updated)
	}
	if w.Expiry != "" {
		fmt.Fprintf(&b, "Expiry: %s\n", w.Expiry)
	}

	if w.Registrar != "" || len(w.NameServers) > 0 {
		b.WriteByte('\n')
	}
	if w.Registrar != "" {
		fmt.Fprintf(&b, "Registrar: %s\n", w.Registrar)
	}
	if len(w.NameServers) > 0 {
		b.WriteString("Name servers:\n")
		for _, ns := range w.NameServers {
			fmt.Fprintf(&b, "  %s\n", ns)
		}
	}

	return strings.TrimSuffix(b.String(), "\n")
}
