package panel

import (
	"fmt"
	"politicaldissidence/data"
	"strings"
)

const whoisCheckedLayout = "06-01-02 15:04"

// DrawWhoisPanel renders the Domain Information panel for a domain's latest WHOIS and DNS records.
func DrawWhoisPanel(hostname, mpName string, w *data.WhoisRecord, dns *data.DnsRecord) string {
	if w == nil && dns == nil {
		return "No WHOIS lookup yet"
	}

	var b strings.Builder
	if w != nil {
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
		} else {
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
		}
	} else {
		host := hostname
		if host == "" {
			host = "(unknown)"
		}
		fmt.Fprintf(&b, "%s\n", host)
		if mpName != "" {
			fmt.Fprintf(&b, "MP: %s\n", mpName)
		}
		b.WriteString("No WHOIS lookup yet\n")
	}

	appendDnsSection(&b, dns)
	return strings.TrimSuffix(b.String(), "\n")
}

func appendDnsSection(b *strings.Builder, dns *data.DnsRecord) {
	if dns == nil {
		b.WriteString("\nDNS: not checked yet")
		return
	}

	b.WriteByte('\n')
	when := ""
	if !dns.CheckedAt.IsZero() {
		when = " (" + dns.CheckedAt.Format(whoisCheckedLayout) + ")"
	}
	if dns.Error != "" {
		fmt.Fprintf(b, "DNS%s: err\n  Error: %s", when, dns.Error)
		return
	}

	label := dns.Outcome
	if label == "" {
		if dns.Empty {
			label = "empty"
		} else {
			label = "ok"
		}
	}
	fmt.Fprintf(b, "DNS%s: %s\n", when, label)
	fmt.Fprintf(b, "  A: %s\n", formatDnsList(dns.A))
	fmt.Fprintf(b, "  NS: %s\n", formatDnsList(dns.NS))
	fmt.Fprintf(b, "  MX: %s", formatDnsList(dns.MX))
	if len(dns.TXT) > 0 {
		fmt.Fprintf(b, "\n  TXT: %s", formatDnsList(dns.TXT))
	}
}

func formatDnsList(vals []string) string {
	if len(vals) == 0 {
		return "(none)"
	}
	return strings.Join(vals, ", ")
}

// DnsMarker returns the domain-list DNS status marker.
func DnsMarker(dns *data.DnsRecord) string {
	if dns == nil {
		return "dns:?"
	}
	if dns.Error != "" {
		return "dns:err"
	}
	if dns.Empty || dns.Outcome == "empty" || dns.Outcome == "nxdomain" {
		return "dns:empty"
	}
	return "dns:ok"
}
