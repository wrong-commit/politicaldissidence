package dnscheck

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"
)

type stubQuerier struct {
	hostSeen string
	ips      []net.IP
	ipErr    error
	ns       []*net.NS
	nsErr    error
	mx       []*net.MX
	mxErr    error
	txt      []string
	txtErr   error
}

func (s *stubQuerier) LookupIP(ctx context.Context, host string) ([]net.IP, error) {
	s.hostSeen = host
	return s.ips, s.ipErr
}
func (s *stubQuerier) LookupNS(ctx context.Context, host string) ([]*net.NS, error) {
	s.hostSeen = host
	return s.ns, s.nsErr
}
func (s *stubQuerier) LookupMX(ctx context.Context, host string) ([]*net.MX, error) {
	s.hostSeen = host
	return s.mx, s.mxErr
}
func (s *stubQuerier) LookupTXT(ctx context.Context, host string) ([]string, error) {
	s.hostSeen = host
	return s.txt, s.txtErr
}

func TestLookupWith_OK(t *testing.T) {
	q := &stubQuerier{
		ips: []net.IP{net.ParseIP("1.2.3.4")},
		ns:  []*net.NS{{Host: "ns1.example.net."}},
		mx:  []*net.MX{{Host: "mail.example.net.", Pref: 10}},
		txt: []string{"v=spf1"},
	}
	info, err := LookupWith("example.com", q, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if info.Outcome != OutcomeOK || info.Empty {
		t.Fatalf("got %+v", info)
	}
	if len(info.A) != 1 || info.A[0] != "1.2.3.4" {
		t.Fatalf("A=%v", info.A)
	}
	if len(info.NS) != 1 || info.NS[0] != "ns1.example.net" {
		t.Fatalf("NS=%v", info.NS)
	}
}

func TestLookupWith_Empty(t *testing.T) {
	q := &stubQuerier{}
	info, err := LookupWith("empty.example", q, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if info.Outcome != OutcomeEmpty || !info.Empty {
		t.Fatalf("got %+v", info)
	}
}

func TestLookupWith_NXDomain(t *testing.T) {
	nf := &net.DNSError{Err: "no such host", IsNotFound: true}
	q := &stubQuerier{ipErr: nf, nsErr: nf, mxErr: nf, txtErr: nf}
	info, err := LookupWith("missing.example", q, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if info.Outcome != OutcomeNXDomain || !info.Empty {
		t.Fatalf("got %+v", info)
	}
}

func TestLookupWith_HardError(t *testing.T) {
	timeout := errors.New("i/o timeout")
	q := &stubQuerier{ipErr: timeout, nsErr: timeout, mxErr: timeout, txtErr: timeout}
	info, err := LookupWith("slow.example", q, time.Second)
	if err == nil {
		t.Fatal("expected error")
	}
	if info.Outcome != OutcomeError || info.Empty {
		t.Fatalf("got %+v", info)
	}
	if !strings.Contains(info.Message, "Could not get DNS") {
		t.Fatalf("message=%q", info.Message)
	}
}

func TestLookupWith_APresentNSEmpty(t *testing.T) {
	q := &stubQuerier{
		ips:   []net.IP{net.ParseIP("9.9.9.9")},
		nsErr: &net.DNSError{Err: "no such host", IsNotFound: true},
	}
	info, err := LookupWith("partial.example", q, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if info.Outcome != OutcomeOK || info.Empty {
		t.Fatalf("got %+v", info)
	}
}

func TestLookupWith_CleansWWW(t *testing.T) {
	q := &stubQuerier{ips: []net.IP{net.ParseIP("1.1.1.1")}}
	_, err := LookupWith("www.example.com", q, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if q.hostSeen != "example.com" {
		t.Fatalf("hostSeen=%q", q.hostSeen)
	}
}

func TestLookupWith_TXTTruncation(t *testing.T) {
	long := strings.Repeat("a", MaxTXTChars+50)
	q := &stubQuerier{txt: []string{long, "short"}}
	info, err := LookupWith("txt.example", q, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if info.Outcome != OutcomeOK || info.Empty {
		t.Fatalf("got %+v", info)
	}
	if len(info.TXT[0]) != MaxTXTChars {
		t.Fatalf("len=%d want %d", len(info.TXT[0]), MaxTXTChars)
	}
	if info.TXT[1] != "short" {
		t.Fatalf("TXT[1]=%q", info.TXT[1])
	}
}

func TestPublicResolverConfigured(t *testing.T) {
	if len(PublicResolvers) < 2 {
		t.Fatal("expected fixed public resolvers")
	}
	if PublicResolvers[0] != "1.1.1.1:53" || PublicResolvers[1] != "8.8.8.8:53" {
		t.Fatalf("PublicResolvers=%v", PublicResolvers)
	}
	r := publicResolver()
	if r == nil || !r.PreferGo || r.Dial == nil {
		t.Fatal("publicResolver must PreferGo with Dial")
	}
}

func TestClean(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"www.example.com", "example.com"},
		{"example.com", "example.com"},
		{"wwwexample.com", "wwwexample.com"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := clean(tt.in); got != tt.want {
			t.Fatalf("clean(%q)=%q want %q", tt.in, got, tt.want)
		}
	}
}
