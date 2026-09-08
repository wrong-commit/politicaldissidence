package dnscheck

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"
)

const (
	// MaxTXTChars is the per-string cap when storing TXT values.
	MaxTXTChars = 250
	// DefaultTimeout bounds each DNS lookup.
	DefaultTimeout = 5 * time.Second
)

// Outcome classifies a DNS emptiness check.
type Outcome string

const (
	OutcomeOK       Outcome = "ok"
	OutcomeEmpty    Outcome = "empty"
	OutcomeNXDomain Outcome = "nxdomain"
	OutcomeError    Outcome = "error"
)

// Info holds live DNS lookup results for a hostname.
type Info struct {
	Hostname string
	Outcome  Outcome
	Empty    bool
	A        []string
	NS       []string
	MX       []string
	TXT      []string
	Message  string
}

// Querier performs DNS lookups (injectable for tests).
type Querier interface {
	LookupIP(ctx context.Context, host string) ([]net.IP, error)
	LookupNS(ctx context.Context, host string) ([]*net.NS, error)
	LookupMX(ctx context.Context, host string) ([]*net.MX, error)
	LookupTXT(ctx context.Context, host string) ([]string, error)
}

// Lookup returns DNS info for hostname using fixed public resolvers.
func Lookup(hostname string) (Info, error) {
	return LookupWith(hostname, defaultQuerier(), DefaultTimeout)
}

// LookupWith returns DNS info using the provided querier (for tests).
// On hard failure Outcome is error, Empty is false, Message is set, and err is non-nil.
// On ok / empty / nxdomain, err is nil.
func LookupWith(hostname string, q Querier, timeout time.Duration) (Info, error) {
	info := Info{Hostname: hostname}
	if q == nil {
		q = defaultQuerier()
	}
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	host := clean(hostname)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var hardErr error
	nxdomain := false

	ips, err := q.LookupIP(ctx, host)
	if err != nil {
		if isNotFound(err) {
			nxdomain = true
		} else {
			hardErr = err
		}
	} else {
		for _, ip := range ips {
			info.A = append(info.A, ip.String())
		}
	}

	nss, err := q.LookupNS(ctx, host)
	if err != nil {
		if isNotFound(err) {
			nxdomain = true
		} else if hardErr == nil {
			hardErr = err
		}
	} else {
		for _, ns := range nss {
			if ns != nil && ns.Host != "" {
				info.NS = append(info.NS, strings.TrimSuffix(ns.Host, "."))
			}
		}
	}

	mxs, err := q.LookupMX(ctx, host)
	if err != nil {
		if isNotFound(err) {
			nxdomain = true
		} else if hardErr == nil {
			hardErr = err
		}
	} else {
		for _, mx := range mxs {
			if mx != nil && mx.Host != "" {
				info.MX = append(info.MX, strings.TrimSuffix(mx.Host, "."))
			}
		}
	}

	txts, err := q.LookupTXT(ctx, host)
	if err != nil {
		if isNotFound(err) {
			nxdomain = true
		} else if hardErr == nil {
			hardErr = err
		}
	} else {
		info.TXT = truncateTXT(txts)
	}

	hasRecords := len(info.A) > 0 || len(info.NS) > 0 || len(info.MX) > 0 || len(info.TXT) > 0
	if hasRecords {
		info.Outcome = OutcomeOK
		info.Empty = false
		return info, nil
	}

	if hardErr != nil {
		info.Outcome = OutcomeError
		info.Empty = false
		info.Message = fmt.Sprintf("Could not get DNS for <%s>: %v", hostname, hardErr)
		return info, hardErr
	}

	if nxdomain {
		info.Outcome = OutcomeNXDomain
		info.Empty = true
		return info, nil
	}

	info.Outcome = OutcomeEmpty
	info.Empty = true
	return info, nil
}

func truncateTXT(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, len(in))
	for i, s := range in {
		if len(s) > MaxTXTChars {
			out[i] = s[:MaxTXTChars]
		} else {
			out[i] = s
		}
	}
	return out
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	if dnsErr, ok := err.(*net.DNSError); ok {
		return dnsErr.IsNotFound
	}
	// Fallback for wrapped / platform messages.
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no such host") || strings.Contains(msg, "nxdomain")
}

func clean(h string) string {
	if strings.HasPrefix(h, "www.") {
		h = strings.Replace(h, "www.", "", 1)
	}
	return h
}

// PublicResolvers are the fixed DNS servers used for live lookups.
var PublicResolvers = []string{"1.1.1.1:53", "8.8.8.8:53"}

type resolverQuerier struct {
	r *net.Resolver
}

func defaultQuerier() Querier {
	return &resolverQuerier{r: publicResolver()}
}

func publicResolver() *net.Resolver {
	return &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			d := net.Dialer{Timeout: 3 * time.Second}
			var last error
			for _, addr := range PublicResolvers {
				c, err := d.DialContext(ctx, network, addr)
				if err == nil {
					return c, nil
				}
				last = err
			}
			if last == nil {
				last = fmt.Errorf("no public resolvers configured")
			}
			return nil, last
		},
	}
}

func (q *resolverQuerier) LookupIP(ctx context.Context, host string) ([]net.IP, error) {
	return q.r.LookupIP(ctx, "ip", host)
}

func (q *resolverQuerier) LookupNS(ctx context.Context, host string) ([]*net.NS, error) {
	return q.r.LookupNS(ctx, host)
}

func (q *resolverQuerier) LookupMX(ctx context.Context, host string) ([]*net.MX, error) {
	return q.r.LookupMX(ctx, host)
}

func (q *resolverQuerier) LookupTXT(ctx context.Context, host string) ([]string, error) {
	return q.r.LookupTXT(ctx, host)
}
