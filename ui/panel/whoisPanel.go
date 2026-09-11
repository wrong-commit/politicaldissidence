package panel

import (
	"fmt"
	"politicaldissidence/data"
	"politicaldissidence/registrarcheck"
	"sort"
	"strings"
)

const whoisCheckedLayout = "06-01-02 15:04"
const httpsExpiryLayout = "2006-01-02"

// DrawWhoisPanel renders the Domain Information panel for a domain's latest WHOIS, HTTPS, DNS, and registrar records.
// When alertReasons is non-empty, those sniping reasons are shown first, separated by ====== from the rest.
// Registrar lines sit at the top of lookup content (after Alert Details).
func DrawWhoisPanel(hostname, mpName string, w *data.WhoisRecord, https *data.HttpsRecord, dns *data.DnsRecord, registrar map[string]*data.RegistrarLookupRecord, alertReasons []string) string {
	hasRegistrar := len(registrar) > 0
	if w == nil && https == nil && dns == nil && !hasRegistrar && len(alertReasons) == 0 {
		return "No domain lookup yet"
	}

	var b strings.Builder
	if len(alertReasons) > 0 {
		b.WriteString("==[Alert Details]==\n")
		for _, r := range alertReasons {
			fmt.Fprintf(&b, "%s\n", r)
		}
		b.WriteString("===================\n")
	}

	if w == nil && https == nil && dns == nil && !hasRegistrar {
		b.WriteString("No domain lookup yet")
		return b.String()
	}

	appendRegistrarSection(&b, registrar)

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
	} else if https != nil || dns != nil {
		host := hostname
		if host == "" {
			host = "(unknown)"
		}
		fmt.Fprintf(&b, "%s\n", host)
		if mpName != "" {
			fmt.Fprintf(&b, "MP: %s\n", mpName)
		}
		b.WriteString("No domain lookup yet\n")
	} else if hasRegistrar {
		host := hostname
		if host == "" {
			host = "(unknown)"
		}
		fmt.Fprintf(&b, "%s\n", host)
		if mpName != "" {
			fmt.Fprintf(&b, "MP: %s\n", mpName)
		}
	}

	appendHttpsSection(&b, https)
	appendDnsSection(&b, dns)
	return strings.TrimSuffix(b.String(), "\n")
}

func appendRegistrarSection(b *strings.Builder, registrar map[string]*data.RegistrarLookupRecord) {
	if len(registrar) == 0 {
		return
	}
	keys := make([]string, 0, len(registrar))
	for k, rec := range registrar {
		if rec == nil {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, src := range keys {
		rec := registrar[src]
		purchaseable := rec.Purchaseable
		if purchaseable == "" {
			purchaseable = registrarcheck.TriWeird
		}
		weird := rec.WeirdResponse
		if weird == "" {
			weird = registrarcheck.TriWeird
		}
		when := ""
		if !rec.CheckedAt.IsZero() {
			when = " (" + rec.CheckedAt.Format(whoisCheckedLayout) + ")"
		}
		fmt.Fprintf(b, "%s%s: purchaseable=%s  weird=%s\n", registrarcheck.SourceLabel(src), when, purchaseable, weird)
		if rec.Error != "" && (weird == registrarcheck.TriYes || weird == registrarcheck.TriWeird) {
			errMsg := rec.Error
			if len(errMsg) > 120 {
				errMsg = errMsg[:120]
			}
			fmt.Fprintf(b, "  Error: %s\n", errMsg)
		}
	}
}

func appendHttpsSection(b *strings.Builder, https *data.HttpsRecord) {
	b.WriteByte('\n')
	if https == nil {
		b.WriteString("HTTPS Status: not checked yet\n")
		b.WriteString("HTTP Status: -\n")
		b.WriteString("Certificate Expiry: -\n")
		return
	}

	status := https.Status
	if status == "" {
		status = "missing"
	}
	fmt.Fprintf(b, "HTTPS Status: %s\n", status)
	if https.HTTPStatus == 0 {
		b.WriteString("HTTP Status: -\n")
	} else {
		fmt.Fprintf(b, "HTTP Status: %d\n", https.HTTPStatus)
	}
	if status == "missing" || https.NotAfter.IsZero() {
		b.WriteString("Certificate Expiry: -\n")
		return
	}
	fmt.Fprintf(b, "Certificate Expiry: %s\n", https.NotAfter.Format(httpsExpiryLayout))
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
