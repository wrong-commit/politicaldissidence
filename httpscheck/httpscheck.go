package httpscheck

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	// DefaultTimeout bounds each TLS dial and HTTP page fetch.
	DefaultTimeout = 5 * time.Second
	// DefaultSoonWindow is how far ahead a leaf NotAfter counts as StatusSoon.
	// Let's Encrypt typically renews around 30 days before expiry, so 15 days is a
	// useful neglect signal (renewal should already have happened).
	DefaultSoonWindow = 15 * 24 * time.Hour
)

// Status classifies a host or aggregated HTTPS check.
type Status string

const (
	StatusEnabled Status = "enabled"
	StatusSoon    Status = "soon"
	StatusExpired Status = "expired"
	StatusMissing Status = "missing"
)

// HostInfo is the TLS + HTTP page result for one hostname (apex or www).
type HostInfo struct {
	Hostname   string
	Status     Status
	NotAfter   time.Time // zero when missing / unknown
	Message    string    // dial / hard failure for this host
	HTTPStatus int       // response status from GET /; 0 when fetch failed
}

// Info holds dual-host HTTPS probe results and the aggregate.
type Info struct {
	Apex       HostInfo
	WWW        HostInfo
	Status     Status    // aggregated (best of Apex, WWW)
	NotAfter   time.Time // from winning host; zero when overall missing
	Message    string    // optional roll-up when overall missing
	HTTPStatus int       // aggregated page status; 0 when neither host returned a code
}

// Dialer performs a TLS dial for a hostname (injectable for tests).
// On success, PeerCertificates should include the leaf at index 0.
// Dial errors and empty peer certs are classified as missing.
type Dialer interface {
	Dial(hostname string, timeout time.Duration) (tls.ConnectionState, error)
}

// PageFetcher GETs https://hostname/ and returns the HTTP status code (injectable for tests).
type PageFetcher interface {
	Fetch(hostname string, timeout time.Duration) (statusCode int, err error)
}

// Lookup probes apex and www.+apex on :443 using the default dialer/fetcher and DefaultSoonWindow.
func Lookup(hostname string) (Info, error) {
	return LookupWith(hostname, defaultDialer(), defaultPageFetcher(), DefaultTimeout, time.Now, DefaultSoonWindow)
}

// LookupWith probes apex and www using the provided dialer and page fetcher (for tests).
// soonWindow controls StatusSoon vs StatusEnabled; <=0 uses DefaultSoonWindow.
// Transport outcomes always yield Info; err is reserved for unexpected failures.
func LookupWith(hostname string, d Dialer, pf PageFetcher, timeout time.Duration, now func() time.Time, soonWindow time.Duration) (Info, error) {
	if d == nil {
		d = defaultDialer()
	}
	if pf == nil {
		pf = defaultPageFetcher()
	}
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	if now == nil {
		now = time.Now
	}
	if soonWindow <= 0 {
		soonWindow = DefaultSoonWindow
	}

	apex := clean(hostname)
	wwwHost := "www." + apex
	at := now()

	apexInfo := probeHost(d, pf, apex, timeout, at, soonWindow)
	wwwInfo := probeHost(d, pf, wwwHost, timeout, at, soonWindow)

	info := Info{
		Apex: apexInfo,
		WWW:  wwwInfo,
	}
	info.Status, info.NotAfter = aggregate(apexInfo, wwwInfo)
	info.HTTPStatus = aggregateHTTP(apexInfo, wwwInfo)
	if info.Status == StatusMissing {
		info.Message = joinMessages(apexInfo.Message, wwwInfo.Message)
	}
	return info, nil
}

func probeHost(d Dialer, pf PageFetcher, hostname string, timeout time.Duration, at time.Time, soonWindow time.Duration) HostInfo {
	h := HostInfo{Hostname: hostname, Status: StatusMissing}
	state, err := d.Dial(hostname, timeout)
	if err != nil {
		h.Message = err.Error()
	} else if len(state.PeerCertificates) == 0 {
		h.Message = "no peer certificates"
	} else {
		leaf := state.PeerCertificates[0]
		h.NotAfter = leaf.NotAfter
		h.Status = classifyNotAfter(leaf.NotAfter, at, soonWindow)
	}

	code, ferr := pf.Fetch(hostname, timeout)
	if ferr != nil {
		if h.Message == "" {
			h.Message = ferr.Error()
		}
		return h
	}
	h.HTTPStatus = code
	return h
}

// classifyNotAfter maps leaf expiry to enabled / soon / expired.
func classifyNotAfter(notAfter, at time.Time, soonWindow time.Duration) Status {
	if !notAfter.After(at) {
		return StatusExpired
	}
	if !notAfter.After(at.Add(soonWindow)) {
		return StatusSoon
	}
	return StatusEnabled
}

func aggregate(apex, www HostInfo) (Status, time.Time) {
	best := apex
	if statusRank(www.Status) > statusRank(apex.Status) {
		best = www
	} else if www.Status == apex.Status && www.Status != StatusMissing {
		// Same tier: prefer earlier NotAfter.
		if !www.NotAfter.IsZero() && (best.NotAfter.IsZero() || www.NotAfter.Before(best.NotAfter)) {
			best = www
		}
	}
	if best.Status == StatusMissing {
		return StatusMissing, time.Time{}
	}
	return best.Status, best.NotAfter
}

// aggregateHTTP prefers 500 then 404 (abandonment signals), else any non-zero code.
func aggregateHTTP(apex, www HostInfo) int {
	for _, h := range []HostInfo{apex, www} {
		if h.HTTPStatus == http.StatusInternalServerError {
			return http.StatusInternalServerError
		}
	}
	for _, h := range []HostInfo{apex, www} {
		if h.HTTPStatus == http.StatusNotFound {
			return http.StatusNotFound
		}
	}
	if apex.HTTPStatus != 0 {
		return apex.HTTPStatus
	}
	return www.HTTPStatus
}

func statusRank(s Status) int {
	switch s {
	case StatusEnabled:
		return 3
	case StatusSoon:
		return 2
	case StatusExpired:
		return 1
	default:
		return 0
	}
}

func joinMessages(a, b string) string {
	switch {
	case a == "" && b == "":
		return ""
	case a == "":
		return b
	case b == "":
		return a
	default:
		return a + "; " + b
	}
}

// clean strips one leading www. and lowercases the hostname.
func clean(h string) string {
	h = strings.TrimSpace(h)
	h = strings.ToLower(h)
	if strings.HasPrefix(h, "www.") {
		h = strings.TrimPrefix(h, "www.")
	}
	return h
}

type tlsDialer struct{}

func defaultDialer() Dialer {
	return tlsDialer{}
}

func (tlsDialer) Dial(hostname string, timeout time.Duration) (tls.ConnectionState, error) {
	dialer := &net.Dialer{Timeout: timeout}
	conn, err := tls.DialWithDialer(dialer, "tcp", net.JoinHostPort(hostname, "443"), &tls.Config{
		ServerName:         hostname,
		InsecureSkipVerify: true, // inspect leaf even when chain/name is untrusted
		MinVersion:         tls.VersionTLS12,
	})
	if err != nil {
		return tls.ConnectionState{}, fmt.Errorf("tls dial %s: %w", hostname, err)
	}
	defer conn.Close()
	return conn.ConnectionState(), nil
}

type httpPageFetcher struct{}

func defaultPageFetcher() PageFetcher {
	return httpPageFetcher{}
}

func (httpPageFetcher) Fetch(hostname string, timeout time.Duration) (int, error) {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
				MinVersion:         tls.VersionTLS12,
			},
		},
	}
	url := "https://" + hostname + "/"
	resp, err := client.Get(url)
	if err != nil {
		return 0, fmt.Errorf("http get %s: %w", hostname, err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
	return resp.StatusCode, nil
}
