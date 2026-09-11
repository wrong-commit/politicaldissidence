package registrarcheck

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// Tri-state values persisted on Domain.RegistrarLookups.
const (
	TriYes   = "yes"
	TriNo    = "no"
	TriWeird = "weird"
)

const (
	SourceGoDaddy   = "godaddy"
	SourceNamecheap = "namecheap"
)

// Env credentials for GoDaddy sso-key auth.
const (
	EnvGoDaddyAPIKey    = "GODADDY_API_KEY"
	EnvGoDaddyAPISecret = "GODADDY_API_SECRET"
)

// ErrNotImplemented is returned for configured sources without a client.
var ErrNotImplemented = errors.New("not implemented")

// Info is the classified registrar availability result.
type Info struct {
	Source        string
	Hostname      string
	Purchaseable  string // yes | no | weird
	WeirdResponse string // yes | no | weird
	Message       string
}

// IsImplemented reports whether a source id has a live client.
func IsImplemented(source string) bool {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case SourceGoDaddy:
		return true
	default:
		return false
	}
}

// SourceLabel returns a pretty name for the Domain Information panel.
func SourceLabel(source string) string {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case SourceGoDaddy:
		return "GoDaddy"
	case SourceNamecheap:
		return "Namecheap"
	default:
		return source
	}
}

// Lookup runs the availability check for source. Unimplemented sources return ErrNotImplemented.
func Lookup(hostname, source string) (Info, error) {
	return LookupWith(hostname, source, nil, nil)
}

// LookupWith is like Lookup with an injectable HTTP client and credential provider.
// creds may be nil (reads env). client may be nil (uses http.DefaultClient with timeout).
func LookupWith(hostname, source string, client *http.Client, creds func() (key, secret string, ok bool)) (Info, error) {
	source = strings.ToLower(strings.TrimSpace(source))
	host := clean(hostname)
	info := Info{Source: source, Hostname: host, Purchaseable: TriWeird, WeirdResponse: TriYes}

	if !IsImplemented(source) {
		info.Message = "not implemented"
		return info, ErrNotImplemented
	}

	switch source {
	case SourceGoDaddy:
		return lookupGoDaddy(host, client, creds)
	default:
		info.Message = "not implemented"
		return info, ErrNotImplemented
	}
}

func clean(h string) string {
	h = strings.ToLower(strings.TrimSpace(h))
	if strings.HasPrefix(h, "www.") {
		h = strings.TrimPrefix(h, "www.")
	}
	return strings.TrimSuffix(h, ".")
}

func defaultCreds() (key, secret string, ok bool) {
	key = strings.TrimSpace(os.Getenv(EnvGoDaddyAPIKey))
	secret = strings.TrimSpace(os.Getenv(EnvGoDaddyAPISecret))
	ok = key != "" && secret != ""
	return key, secret, ok
}

func defaultClient() *http.Client {
	return &http.Client{Timeout: 15 * time.Second}
}

func FormatNotImplemented(source, hostname string) string {
	return fmt.Sprintf("ERROR registrar %s %s: not implemented", source, hostname)
}

func FormatLookupError(source, hostname, reason string) string {
	return fmt.Sprintf("ERROR registrar %s %s: %s", source, hostname, reason)
}

func FormatLookupInfo(source, hostname, purchaseable, weirdResponse string) string {
	return fmt.Sprintf("INFO registrar %s %s purchaseable=%s weirdResponse=%s", source, hostname, purchaseable, weirdResponse)
}

func FormatLookupDebug(mpName, source, hostname string) string {
	return fmt.Sprintf("DEBUG registrar %s checking %s %s", source, mpName, hostname)
}
