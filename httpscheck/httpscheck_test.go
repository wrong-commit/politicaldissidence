package httpscheck

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"math/big"
	"testing"
	"time"
)

type stubDialer struct {
	// results keyed by hostname
	byHost map[string]struct {
		state tls.ConnectionState
		err   error
	}
	seen []string
}

func (s *stubDialer) Dial(hostname string, timeout time.Duration) (tls.ConnectionState, error) {
	s.seen = append(s.seen, hostname)
	if s.byHost == nil {
		return tls.ConnectionState{}, errors.New("no stub")
	}
	r, ok := s.byHost[hostname]
	if !ok {
		return tls.ConnectionState{}, errors.New("unexpected host: " + hostname)
	}
	return r.state, r.err
}

type stubPageFetcher struct {
	byHost map[string]struct {
		code int
		err  error
	}
	seen []string
}

func (s *stubPageFetcher) Fetch(hostname string, timeout time.Duration) (int, error) {
	s.seen = append(s.seen, hostname)
	if s.byHost == nil {
		return 0, nil
	}
	r, ok := s.byHost[hostname]
	if !ok {
		return 0, errors.New("unexpected host: " + hostname)
	}
	return r.code, r.err
}

// okPages returns 200 for apex and www of example.com (common test host).
func okPages() *stubPageFetcher {
	return &stubPageFetcher{byHost: map[string]struct {
		code int
		err  error
	}{
		"example.com":     {code: 200},
		"www.example.com": {code: 200},
	}}
}

func leafCert(notAfter time.Time) *x509.Certificate {
	return &x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotAfter:     notAfter,
	}
}

func stateWithLeaf(notAfter time.Time) tls.ConnectionState {
	return tls.ConnectionState{
		PeerCertificates: []*x509.Certificate{leafCert(notAfter)},
	}
}

func lookup(hostname string, d Dialer, at time.Time, soon time.Duration) (Info, error) {
	return LookupWith(hostname, d, okPages(), time.Second, func() time.Time { return at }, soon)
}

func lookupHTTP(hostname string, d Dialer, pf PageFetcher, at time.Time, soon time.Duration) (Info, error) {
	return LookupWith(hostname, d, pf, time.Second, func() time.Time { return at }, soon)
}

func TestLookupWith_EnabledSoonExpiredMissing(t *testing.T) {
	now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	far := now.Add(120 * 24 * time.Hour)
	near := now.Add(10 * 24 * time.Hour)
	past := now.Add(-24 * time.Hour)
	soon := DefaultSoonWindow

	t.Run("enabled far", func(t *testing.T) {
		d := &stubDialer{byHost: map[string]struct {
			state tls.ConnectionState
			err   error
		}{
			"example.com":     {state: stateWithLeaf(far)},
			"www.example.com": {err: errors.New("refused")},
		}}
		info, err := lookup("example.com", d, now, soon)
		if err != nil {
			t.Fatal(err)
		}
		if info.Apex.Status != StatusEnabled || !info.Apex.NotAfter.Equal(far) {
			t.Fatalf("apex=%+v", info.Apex)
		}
	})

	t.Run("soon within window", func(t *testing.T) {
		d := &stubDialer{byHost: map[string]struct {
			state tls.ConnectionState
			err   error
		}{
			"example.com":     {state: stateWithLeaf(near)},
			"www.example.com": {err: errors.New("refused")},
		}}
		info, err := lookup("example.com", d, now, soon)
		if err != nil {
			t.Fatal(err)
		}
		if info.Apex.Status != StatusSoon || info.Status != StatusSoon {
			t.Fatalf("apex=%+v status=%s", info.Apex, info.Status)
		}
	})

	t.Run("expired", func(t *testing.T) {
		d := &stubDialer{byHost: map[string]struct {
			state tls.ConnectionState
			err   error
		}{
			"example.com":     {state: stateWithLeaf(past)},
			"www.example.com": {err: errors.New("refused")},
		}}
		info, err := lookup("example.com", d, now, soon)
		if err != nil {
			t.Fatal(err)
		}
		if info.Apex.Status != StatusExpired {
			t.Fatalf("apex=%+v", info.Apex)
		}
	})

	t.Run("missing", func(t *testing.T) {
		d := &stubDialer{byHost: map[string]struct {
			state tls.ConnectionState
			err   error
		}{
			"example.com":     {err: errors.New("i/o timeout")},
			"www.example.com": {err: errors.New("refused")},
		}}
		info, err := lookup("example.com", d, now, soon)
		if err != nil {
			t.Fatal(err)
		}
		if info.Apex.Status != StatusMissing || info.Apex.Message == "" {
			t.Fatalf("apex=%+v", info.Apex)
		}
	})
}

func TestLookupWith_SoonWindowConfigurable(t *testing.T) {
	now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	in10 := now.Add(10 * 24 * time.Hour)
	d := &stubDialer{byHost: map[string]struct {
		state tls.ConnectionState
		err   error
	}{
		"example.com":     {state: stateWithLeaf(in10)},
		"www.example.com": {state: stateWithLeaf(in10)},
	}}

	info15, err := lookup("example.com", d, now, 15*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if info15.Status != StatusSoon {
		t.Fatalf("15d window: got %s", info15.Status)
	}

	d.seen = nil
	info7, err := lookup("example.com", d, now, 7*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if info7.Status != StatusEnabled {
		t.Fatalf("7d window: 10d cert should be enabled, got %s", info7.Status)
	}
}

func TestLookupWith_DualHostAndNormalize(t *testing.T) {
	now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	far := now.Add(120 * 24 * time.Hour)
	soon := DefaultSoonWindow

	t.Run("H4 both dialed", func(t *testing.T) {
		d := &stubDialer{byHost: map[string]struct {
			state tls.ConnectionState
			err   error
		}{
			"example.com":     {state: stateWithLeaf(far)},
			"www.example.com": {state: stateWithLeaf(far)},
		}}
		_, err := lookup("example.com", d, now, soon)
		if err != nil {
			t.Fatal(err)
		}
		if len(d.seen) != 2 {
			t.Fatalf("seen=%v", d.seen)
		}
	})

	t.Run("H5 www input", func(t *testing.T) {
		d := &stubDialer{byHost: map[string]struct {
			state tls.ConnectionState
			err   error
		}{
			"example.com":     {state: stateWithLeaf(far)},
			"www.example.com": {state: stateWithLeaf(far)},
		}}
		_, err := lookup("www.example.com", d, now, soon)
		if err != nil {
			t.Fatal(err)
		}
		for _, h := range d.seen {
			if h == "www.www.example.com" {
				t.Fatal("probed www.www")
			}
		}
	})
}

func TestLookupWith_Aggregate(t *testing.T) {
	now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	far := now.Add(120 * 24 * time.Hour)
	farther := now.Add(200 * 24 * time.Hour)
	near := now.Add(10 * 24 * time.Hour)
	soonerNear := now.Add(5 * 24 * time.Hour)
	past := now.Add(-24 * time.Hour)
	earlierPast := now.Add(-48 * time.Hour)
	soon := DefaultSoonWindow

	cases := []struct {
		name       string
		apex       struct {
			state tls.ConnectionState
			err   error
		}
		www struct {
			state tls.ConnectionState
			err   error
		}
		wantStatus Status
		wantAfter  time.Time
	}{
		{
			name: "apex enabled www missing",
			apex: struct {
				state tls.ConnectionState
				err   error
			}{state: stateWithLeaf(far)},
			www: struct {
				state tls.ConnectionState
				err   error
			}{err: errors.New("refused")},
			wantStatus: StatusEnabled,
			wantAfter:  far,
		},
		{
			name: "apex soon www missing",
			apex: struct {
				state tls.ConnectionState
				err   error
			}{state: stateWithLeaf(near)},
			www: struct {
				state tls.ConnectionState
				err   error
			}{err: errors.New("refused")},
			wantStatus: StatusSoon,
			wantAfter:  near,
		},
		{
			name: "apex soon www enabled → enabled",
			apex: struct {
				state tls.ConnectionState
				err   error
			}{state: stateWithLeaf(near)},
			www: struct {
				state tls.ConnectionState
				err   error
			}{state: stateWithLeaf(far)},
			wantStatus: StatusEnabled,
			wantAfter:  far,
		},
		{
			name: "apex expired www soon → soon",
			apex: struct {
				state tls.ConnectionState
				err   error
			}{state: stateWithLeaf(past)},
			www: struct {
				state tls.ConnectionState
				err   error
			}{state: stateWithLeaf(near)},
			wantStatus: StatusSoon,
			wantAfter:  near,
		},
		{
			name: "apex expired www missing",
			apex: struct {
				state tls.ConnectionState
				err   error
			}{state: stateWithLeaf(past)},
			www: struct {
				state tls.ConnectionState
				err   error
			}{err: errors.New("refused")},
			wantStatus: StatusExpired,
			wantAfter:  past,
		},
		{
			name: "both missing",
			apex: struct {
				state tls.ConnectionState
				err   error
			}{err: errors.New("a")},
			www: struct {
				state tls.ConnectionState
				err   error
			}{err: errors.New("b")},
			wantStatus: StatusMissing,
		},
		{
			name: "both enabled earlier NotAfter",
			apex: struct {
				state tls.ConnectionState
				err   error
			}{state: stateWithLeaf(farther)},
			www: struct {
				state tls.ConnectionState
				err   error
			}{state: stateWithLeaf(far)},
			wantStatus: StatusEnabled,
			wantAfter:  far,
		},
		{
			name: "both soon earlier NotAfter",
			apex: struct {
				state tls.ConnectionState
				err   error
			}{state: stateWithLeaf(near)},
			www: struct {
				state tls.ConnectionState
				err   error
			}{state: stateWithLeaf(soonerNear)},
			wantStatus: StatusSoon,
			wantAfter:  soonerNear,
		},
		{
			name: "both expired earlier NotAfter",
			apex: struct {
				state tls.ConnectionState
				err   error
			}{state: stateWithLeaf(past)},
			www: struct {
				state tls.ConnectionState
				err   error
			}{state: stateWithLeaf(earlierPast)},
			wantStatus: StatusExpired,
			wantAfter:  earlierPast,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			d := &stubDialer{byHost: map[string]struct {
				state tls.ConnectionState
				err   error
			}{
				"example.com":     tt.apex,
				"www.example.com": tt.www,
			}}
			info, err := lookup("example.com", d, now, soon)
			if err != nil {
				t.Fatal(err)
			}
			if info.Status != tt.wantStatus {
				t.Fatalf("Status=%s want %s", info.Status, tt.wantStatus)
			}
			if tt.wantStatus == StatusMissing {
				if !info.NotAfter.IsZero() {
					t.Fatalf("NotAfter should be zero, got %v", info.NotAfter)
				}
				return
			}
			if !info.NotAfter.Equal(tt.wantAfter) {
				t.Fatalf("NotAfter=%v want %v", info.NotAfter, tt.wantAfter)
			}
		})
	}
}

func TestLookupWith_SelfSignedEnabled(t *testing.T) {
	now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	far := now.Add(120 * 24 * time.Hour)
	d := &stubDialer{byHost: map[string]struct {
		state tls.ConnectionState
		err   error
	}{
		"example.com":     {state: stateWithLeaf(far)},
		"www.example.com": {state: stateWithLeaf(far)},
	}}
	info, err := lookup("example.com", d, now, DefaultSoonWindow)
	if err != nil {
		t.Fatal(err)
	}
	if info.Status != StatusEnabled || info.Apex.Status != StatusEnabled {
		t.Fatalf("got %+v", info)
	}
}

func TestLookupWith_EmptyPeerCerts(t *testing.T) {
	now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	d := &stubDialer{byHost: map[string]struct {
		state tls.ConnectionState
		err   error
	}{
		"example.com":     {state: tls.ConnectionState{}},
		"www.example.com": {state: tls.ConnectionState{}},
	}}
	info, err := lookup("example.com", d, now, DefaultSoonWindow)
	if err != nil {
		t.Fatal(err)
	}
	if info.Status != StatusMissing {
		t.Fatalf("got %+v", info)
	}
}

func TestClean(t *testing.T) {
	if got := clean("WWW.Example.COM"); got != "example.com" {
		t.Fatalf("got %q", got)
	}
}

func TestLookupWith_HTTPStatus(t *testing.T) {
	now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	far := now.Add(120 * 24 * time.Hour)
	d := &stubDialer{byHost: map[string]struct {
		state tls.ConnectionState
		err   error
	}{
		"example.com":     {state: stateWithLeaf(far)},
		"www.example.com": {state: stateWithLeaf(far)},
	}}

	t.Run("prefers 500 over 200", func(t *testing.T) {
		pf := &stubPageFetcher{byHost: map[string]struct {
			code int
			err  error
		}{
			"example.com":     {code: 200},
			"www.example.com": {code: 500},
		}}
		info, err := lookupHTTP("example.com", d, pf, now, DefaultSoonWindow)
		if err != nil {
			t.Fatal(err)
		}
		if info.HTTPStatus != 500 || info.WWW.HTTPStatus != 500 || info.Apex.HTTPStatus != 200 {
			t.Fatalf("got %+v", info)
		}
	})

	t.Run("prefers 404 over 200", func(t *testing.T) {
		pf := &stubPageFetcher{byHost: map[string]struct {
			code int
			err  error
		}{
			"example.com":     {code: 404},
			"www.example.com": {code: 200},
		}}
		info, err := lookupHTTP("example.com", d, pf, now, DefaultSoonWindow)
		if err != nil {
			t.Fatal(err)
		}
		if info.HTTPStatus != 404 {
			t.Fatalf("HTTPStatus=%d", info.HTTPStatus)
		}
	})

	t.Run("prefers 500 over 404", func(t *testing.T) {
		pf := &stubPageFetcher{byHost: map[string]struct {
			code int
			err  error
		}{
			"example.com":     {code: 404},
			"www.example.com": {code: 500},
		}}
		info, err := lookupHTTP("example.com", d, pf, now, DefaultSoonWindow)
		if err != nil {
			t.Fatal(err)
		}
		if info.HTTPStatus != 500 {
			t.Fatalf("HTTPStatus=%d", info.HTTPStatus)
		}
	})

	t.Run("fetch error yields zero", func(t *testing.T) {
		pf := &stubPageFetcher{byHost: map[string]struct {
			code int
			err  error
		}{
			"example.com":     {err: errors.New("timeout")},
			"www.example.com": {err: errors.New("refused")},
		}}
		info, err := lookupHTTP("example.com", d, pf, now, DefaultSoonWindow)
		if err != nil {
			t.Fatal(err)
		}
		if info.HTTPStatus != 0 || info.Apex.HTTPStatus != 0 {
			t.Fatalf("got %+v", info)
		}
	})

	t.Run("both hosts fetched", func(t *testing.T) {
		pf := okPages()
		_, err := lookupHTTP("example.com", d, pf, now, DefaultSoonWindow)
		if err != nil {
			t.Fatal(err)
		}
		if len(pf.seen) != 2 {
			t.Fatalf("seen=%v", pf.seen)
		}
	})
}
