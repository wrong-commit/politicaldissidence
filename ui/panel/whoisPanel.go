package panel

import (
	"fmt"
	"strings"
	"time"
)

const whoisCheckedLayout = "06-01-02 15:04"

// WhoisSnapshot is session-only latest WHOIS detail for the WHOIS panel.
// It is never persisted to JSON.
type WhoisSnapshot struct {
	Hostname    string
	MPName      string
	CheckedAt   time.Time
	Status      []string
	Created     string
	Updated     string
	Expiry      string
	Registrar   string
	NameServers []string
	Error       string
}

// Empty reports whether no lookup has been recorded yet.
func (s *WhoisSnapshot) Empty() bool {
	return s == nil || (s.Hostname == "" && s.Error == "" && s.CheckedAt.IsZero())
}

// DrawWhoisPanel renders the WHOIS information panel buffer.
func DrawWhoisPanel(s *WhoisSnapshot) string {
	if s.Empty() {
		return "No WHOIS lookup yet"
	}

	var b strings.Builder
	host := s.Hostname
	if host == "" {
		host = "(unknown)"
	}
	fmt.Fprintf(&b, "%s\n", host)

	if s.Error != "" {
		fmt.Fprintf(&b, "Error: %s\n", s.Error)
		if !s.CheckedAt.IsZero() {
			fmt.Fprintf(&b, "Checked: %s\n", s.CheckedAt.Format(whoisCheckedLayout))
		}
		if s.MPName != "" {
			fmt.Fprintf(&b, "MP: %s\n", s.MPName)
		}
		return strings.TrimSuffix(b.String(), "\n")
	}

	if s.MPName != "" {
		fmt.Fprintf(&b, "MP: %s\n", s.MPName)
	}
	if !s.CheckedAt.IsZero() {
		fmt.Fprintf(&b, "Checked: %s\n", s.CheckedAt.Format(whoisCheckedLayout))
	}
	b.WriteByte('\n')

	if len(s.Status) > 0 {
		fmt.Fprintf(&b, "Status: %s\n", strings.Join(s.Status, ", "))
	}
	if s.Created != "" {
		fmt.Fprintf(&b, "Created: %s\n", s.Created)
	}
	if s.Updated != "" {
		fmt.Fprintf(&b, "Updated: %s\n", s.Updated)
	}
	if s.Expiry != "" {
		fmt.Fprintf(&b, "Expiry: %s\n", s.Expiry)
	}

	if s.Registrar != "" || len(s.NameServers) > 0 {
		b.WriteByte('\n')
	}
	if s.Registrar != "" {
		fmt.Fprintf(&b, "Registrar: %s\n", s.Registrar)
	}
	if len(s.NameServers) > 0 {
		b.WriteString("Name servers:\n")
		for _, ns := range s.NameServers {
			fmt.Fprintf(&b, "  %s\n", ns)
		}
	}

	return strings.TrimSuffix(b.String(), "\n")
}
