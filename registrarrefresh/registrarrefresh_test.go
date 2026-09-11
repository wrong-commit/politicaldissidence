package registrarrefresh

import (
	"strings"
	"testing"
	"time"

	"politicaldissidence/data"
	"politicaldissidence/domainlookups"
	"politicaldissidence/registrarcheck"
)

func TestRun_ForceWalksSourcesAndSkipsNamecheap(t *testing.T) {
	var infos []string
	var errorsLogged []string
	cfg := domainlookups.Config{
		"com": []string{"godaddy", "namecheap"},
	}
	mps := []data.MP{{
		FirstName: "Jane",
		Surname:   "Doe",
		Domains:   []data.Domain{{Hostname: "example.com"}},
	}}
	res := Run(mps, Deps{
		Force: true,
		LoadConfig: func() (domainlookups.Config, error) {
			return cfg, nil
		},
		UpdateRegistrarSource: func(d *data.Domain, source string) (registrarcheck.Info, error) {
			if source == "namecheap" {
				return registrarcheck.Info{Message: "not implemented"}, registrarcheck.ErrNotImplemented
			}
			info := registrarcheck.Info{
				Source: source, Hostname: d.Hostname,
				Purchaseable: registrarcheck.TriNo, WeirdResponse: registrarcheck.TriNo,
			}
			if d.RegistrarLookups == nil {
				d.RegistrarLookups = map[string]*data.RegistrarLookupRecord{}
			}
			d.RegistrarLookups[source] = &data.RegistrarLookupRecord{
				CheckedAt: time.Now(), Purchaseable: "no", WeirdResponse: "no",
			}
			return info, nil
		},
		Log: LogFn{
			OnInfo: func(msg string) { infos = append(infos, msg) },
			OnError: func(msg string) { errorsLogged = append(errorsLogged, msg) },
		},
	})
	if res.UpdatedDomains != 1 {
		t.Fatalf("updated=%d", res.UpdatedDomains)
	}
	if _, ok := mps[0].Domains[0].RegistrarLookups["namecheap"]; ok {
		t.Fatal("should not persist namecheap")
	}
	if mps[0].Domains[0].RegistrarLookups["godaddy"] == nil {
		t.Fatal("expected godaddy persist")
	}
	found := false
	for _, e := range errorsLogged {
		if strings.Contains(e, "namecheap") && strings.Contains(e, "not implemented") {
			found = true
		}
	}
	if !found {
		t.Fatalf("errors=%v", errorsLogged)
	}
}

func TestRun_SkipsFresh(t *testing.T) {
	at := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	cfg := domainlookups.Config{"com.au": []string{"godaddy"}}
	mps := []data.MP{{
		Domains: []data.Domain{{
			Hostname: "x.com.au",
			RegistrarLookups: map[string]*data.RegistrarLookupRecord{
				"godaddy": {CheckedAt: at.Add(-time.Hour), Purchaseable: "no", WeirdResponse: "no"},
			},
		}},
	}}
	called := 0
	res := Run(mps, Deps{
		Now:   func() time.Time { return at },
		Force: false,
		LoadConfig: func() (domainlookups.Config, error) {
			return cfg, nil
		},
		UpdateRegistrarSource: func(d *data.Domain, source string) (registrarcheck.Info, error) {
			called++
			return registrarcheck.Info{}, nil
		},
		Log: LogFn{},
	})
	if called != 0 || res.UpdatedDomains != 0 {
		t.Fatalf("called=%d updated=%d", called, res.UpdatedDomains)
	}
}
