package csvrefresh

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"politicaldissidence/csv"
	"politicaldissidence/data"
)

type memLog struct {
	mu   sync.Mutex
	info []string
	dbg  []string
	err  []string
}

func (m *memLog) Info(msg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.info = append(m.info, msg)
}
func (m *memLog) Debug(msg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dbg = append(m.dbg, msg)
}
func (m *memLog) Error(msg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = append(m.err, msg)
}

func senatorCSV() string {
	return "Title,First Name,Surname,Other Name,Preferred Name,Political Party,State,Electorate Suburb\n" +
		"Sen,Ada,Lovelace,,,ALP,NSW,Sydney\n" +
		"Sen,Grace,Hopper,,,LP,VIC,Melbourne\n"
}

func memberCSV() string {
	return "Honorific,First Name,Surname,Other Name,Preferred Name,Political Party,State,Electorate\n" +
		"Ms,Jane,Doe,,,GRN,QLD,Lilley\n"
}

func customCSV() string {
	return "Title,First Name,Surname,Party,State,District\n" +
		"Hon,Pat,Smith,IND,WA,Perth\n"
}

func validCfg(t *testing.T, formats ...string) *Config {
	t.Helper()
	if len(formats) == 0 {
		formats = []string{FormatSenators}
	}
	parts := make([]string, len(formats))
	for i, f := range formats {
		parts[i] = entryJSON(f)
	}
	c, err := ParseConfig(wrapEntries(parts...))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestRun_AddAndMerge(t *testing.T) {
	log := &memLog{}
	var applied []data.MP
	cfg := validCfg(t, FormatSenators)
	html := `<html><a href="/files/allsenel.csv">csv</a></html>`

	current := []data.MP{{
		FirstName: "Ada", Surname: "Lovelace", Party: "OLD",
		Domains: []data.Domain{{Hostname: "ada.example"}},
	}}

	res := Run(Deps{
		Log:    log,
		Config: cfg,
		Current: func() []data.MP {
			return current
		},
		Get: func(u string) ([]byte, error) {
			if strings.Contains(u, "page") {
				return []byte(html), nil
			}
			return []byte(senatorCSV()), nil
		},
		Apply: func(merged []data.MP) {
			applied = merged
		},
	})

	if res.ParsedCount != 2 || res.AddedCount != 1 || res.MergedCount != 1 {
		t.Fatalf("res=%+v", res)
	}
	if !res.Applied || len(applied) != 2 {
		t.Fatalf("applied=%d appliedFlag=%v", len(applied), res.Applied)
	}
	joined := strings.Join(log.info, "\n")
	if !strings.Contains(joined, `added name="Grace Hopper"`) {
		t.Fatalf("missing added: %s", joined)
	}
	if !strings.Contains(joined, `merged name="Ada Lovelace"`) {
		t.Fatalf("missing merged: %s", joined)
	}
	if !strings.Contains(joined, "unsaved") {
		t.Fatalf("missing unsaved: %s", joined)
	}
}

func TestRun_MultipleEntries(t *testing.T) {
	log := &memLog{}
	var applied []data.MP
	cfg := validCfg(t, FormatSenators, FormatMembers)
	html := `<html>
		<a href="/files/allsenel.csv">s</a>
		<a href="/files/FamilynameRepsCSV.csv">m</a>
	</html>`

	res := Run(Deps{
		Log:    log,
		Config: cfg,
		Current: func() []data.MP {
			return nil
		},
		Get: func(u string) ([]byte, error) {
			if strings.Contains(u, "page") {
				return []byte(html), nil
			}
			if strings.Contains(u, "FamilynameRepsCSV") {
				return []byte(memberCSV()), nil
			}
			return []byte(senatorCSV()), nil
		},
		Apply: func(merged []data.MP) {
			applied = merged
		},
	})

	if res.ParsedCount != 3 || res.AddedCount != 3 || res.EntryFails != 0 {
		t.Fatalf("res=%+v", res)
	}
	if !res.Applied || len(applied) != 3 {
		t.Fatalf("applied=%d flag=%v", len(applied), res.Applied)
	}
	joined := strings.Join(log.info, "\n")
	if !strings.Contains(joined, "entry 1/2") || !strings.Contains(joined, "entry 2/2") {
		t.Fatalf("missing entry logs: %s", joined)
	}
}

func TestRun_UnchangedEmptyCSV(t *testing.T) {
	log := &memLog{}
	cfg := validCfg(t, FormatSenators)
	html := `<html><a href="/files/allsenel.csv">csv</a></html>`
	empty := "Title,First Name,Surname,Other Name,Preferred Name,Political Party,State,Electorate Suburb\n"
	res := Run(Deps{
		Log:    log,
		Config: cfg,
		Current: func() []data.MP {
			return []data.MP{{FirstName: "Ada", Surname: "Lovelace"}}
		},
		Get: func(u string) ([]byte, error) {
			if strings.Contains(u, "page") {
				return []byte(html), nil
			}
			return []byte(empty), nil
		},
		Apply: func([]data.MP) {},
	})
	if res.AddedCount != 0 || res.MergedCount != 0 {
		t.Fatalf("res=%+v", res)
	}
	if log.info[len(log.info)-1] != FormatEndUnchanged() {
		t.Fatalf("end=%q", log.info[len(log.info)-1])
	}
}

func TestRun_DownloadError(t *testing.T) {
	log := &memLog{}
	cfg := validCfg(t, FormatSenators)
	res := Run(Deps{
		Log:    log,
		Config: cfg,
		Get:    func(string) ([]byte, error) { return nil, errors.New("dial fail") },
		Apply:  func([]data.MP) { t.Fatal("must not apply") },
	})
	if res.Applied || res.ParsedCount != 0 || res.EntryFails != 1 {
		t.Fatalf("res=%+v", res)
	}
	if len(log.err) == 0 || !strings.Contains(log.err[0], "dial fail") {
		t.Fatalf("err=%v", log.err)
	}
	if !strings.Contains(log.err[0], "https://example.org/page") {
		t.Fatalf("expected failed URL in log: %v", log.err)
	}
}

func TestRun_ParseError(t *testing.T) {
	log := &memLog{}
	cfg := validCfg(t, FormatSenators)
	html := `<html><a href="/files/allsenel.csv">csv</a></html>`
	res := Run(Deps{
		Log:    log,
		Config: cfg,
		Get: func(u string) ([]byte, error) {
			if strings.Contains(u, "page") {
				return []byte(html), nil
			}
			return []byte("Nope,Wrong\nx,y\n"), nil
		},
		Apply: func([]data.MP) { t.Fatal("must not apply") },
	})
	if res.Applied || res.EntryFails != 1 {
		t.Fatalf("res=%+v", res)
	}
	if len(log.err) == 0 || !strings.Contains(log.err[0], "parse:") {
		t.Fatalf("err=%v", log.err)
	}
}

func TestRun_SecondEntryContinuesAfterFirstFails(t *testing.T) {
	log := &memLog{}
	var applied []data.MP
	cfg := validCfg(t, FormatSenators, FormatMembers)
	html := `<html>
		<a href="/files/allsenel.csv">s</a>
		<a href="/files/FamilynameRepsCSV.csv">m</a>
	</html>`

	res := Run(Deps{
		Log:     log,
		Config:  cfg,
		Current: func() []data.MP { return nil },
		Get: func(u string) ([]byte, error) {
			if strings.Contains(u, "page") {
				return []byte(html), nil
			}
			if strings.Contains(u, "allsenel") {
				return nil, errors.New("senators down")
			}
			return []byte(memberCSV()), nil
		},
		Apply: func(m []data.MP) { applied = m },
	})
	if res.EntryFails != 1 || res.ParsedCount != 1 || res.AddedCount != 1 || !res.Applied {
		t.Fatalf("res=%+v", res)
	}
	if len(applied) != 1 || applied[0].Surname != "Doe" {
		t.Fatalf("applied=%+v", applied)
	}
}

func TestRun_ParseMembers(t *testing.T) {
	cfg := validCfg(t, FormatMembers)
	html := `<html><a href="/files/FamilynameRepsCSV.csv">csv</a></html>`
	var got []data.MP
	res := Run(Deps{
		Config:  cfg,
		Current: func() []data.MP { return nil },
		Get: func(u string) ([]byte, error) {
			if strings.Contains(u, "page") {
				return []byte(html), nil
			}
			return []byte(memberCSV()), nil
		},
		Apply: func(m []data.MP) { got = m },
	})
	if res.ParsedCount != 1 || res.AddedCount != 1 {
		t.Fatalf("res=%+v", res)
	}
	if got[0].Level != data.Level.FedRep || got[0].Electorate != "Lilley" {
		t.Fatalf("%+v", got[0])
	}
}

func TestRun_ParseCustom(t *testing.T) {
	cfg := validCfg(t, FormatCustom)
	html := `<html><a href="/files/roster.csv">csv</a></html>`
	var got []data.MP
	res := Run(Deps{
		Config:  cfg,
		Current: func() []data.MP { return nil },
		Get: func(u string) ([]byte, error) {
			if strings.Contains(u, "page") {
				return []byte(html), nil
			}
			return []byte(customCSV()), nil
		},
		Apply: func(m []data.MP) { got = m },
	})
	if res.ParsedCount != 1 {
		t.Fatalf("res=%+v", res)
	}
	if got[0].Level != "State Senator" || got[0].Electorate != "Perth" {
		t.Fatalf("%+v", got[0])
	}
}

func TestRunner_SingleFlight(t *testing.T) {
	cfg := validCfg(t, FormatSenators)
	html := `<html><a href="/files/allsenel.csv">csv</a></html>`
	started := make(chan struct{})
	release := make(chan struct{})
	var r Runner
	log := &memLog{}

	done := make(chan struct{})
	go func() {
		defer close(done)
		r.TryRun(Deps{
			Log:    log,
			Config: cfg,
			Current: func() []data.MP {
				return nil
			},
			Get: func(u string) ([]byte, error) {
				if strings.Contains(u, "page") {
					close(started)
					<-release
					return []byte(html), nil
				}
				return []byte(senatorCSV()), nil
			},
			Apply: func([]data.MP) {},
		})
	}()
	<-started
	_, ok := r.TryRun(Deps{Log: log, Config: cfg})
	if ok {
		t.Fatal("second should be rejected")
	}
	close(release)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for first run")
	}
	if !containsInfo(log, FormatAlreadyRunning()) {
		t.Fatalf("want already running in %v", log.info)
	}
}

func containsInfo(log *memLog, msg string) bool {
	log.mu.Lock()
	defer log.mu.Unlock()
	for _, s := range log.info {
		if s == msg {
			return true
		}
	}
	return false
}

func TestTryRun_MissingConfig(t *testing.T) {
	log := &memLog{}
	var r Runner
	res, ok := r.TryRun(Deps{Log: log})
	if !ok || !res.Skipped {
		t.Fatalf("res=%+v ok=%v", res, ok)
	}
	if len(log.err) == 0 || !strings.Contains(log.err[0], "config") {
		t.Fatalf("err=%v", log.err)
	}
}

func TestParseReader_EmptyColumnSkipped(t *testing.T) {
	body := "First Name,Surname\nAda,Lovelace\n"
	cols := csv.ColumnMap{FirstName: "First Name", Surname: "Surname"}
	mps, err := csv.ParseReader(strings.NewReader(body), cols, "X")
	if err != nil {
		t.Fatal(err)
	}
	if len(mps) != 1 || mps[0].FirstName != "Ada" || mps[0].Honorific != "" {
		t.Fatalf("%+v", mps)
	}
}
