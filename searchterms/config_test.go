package searchterms

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"politicaldissidence/data"
)

func sampleMP() data.MP {
	return data.MP{
		Honorific:  "Ms",
		FirstName:  "Jane",
		Surname:    "Doe",
		Electorate: "Example",
		Party:      "IND",
	}
}

func TestBuiltinRender(t *testing.T) {
	c := Builtin()
	if c.Len() != 2 {
		t.Fatalf("len=%d", c.Len())
	}
	mp := sampleMP()
	got0, err := c.Render(0, mp)
	if err != nil {
		t.Fatal(err)
	}
	want0 := strings.TrimSpace(mp.SearchTerm1())
	if got0 != want0 {
		t.Fatalf("t1: got %q want %q", got0, want0)
	}
	got1, err := c.Render(1, mp)
	if err != nil {
		t.Fatal(err)
	}
	want1 := strings.TrimSpace(mp.SearchTerm2())
	if got1 != want1 {
		t.Fatalf("t2: got %q want %q", got1, want1)
	}
}

func TestParseConfig(t *testing.T) {
	c, err := ParseConfig([]byte(`{
		"terms": [
			{"id": "a", "label": "A", "template": "{{.FirstName}} {{.Surname}}"},
			{"template": "{{.Party}}"}
		]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if c.Len() != 2 {
		t.Fatalf("len=%d", c.Len())
	}
	if c.Terms[1].ID != "T2" {
		t.Fatalf("default id=%q", c.Terms[1].ID)
	}
	got, err := c.Render(0, sampleMP())
	if err != nil {
		t.Fatal(err)
	}
	if got != "Jane Doe" {
		t.Fatalf("got %q", got)
	}
}

func TestParseConfigErrors(t *testing.T) {
	cases := []string{
		`{}`,
		`{"terms":[]}`,
		`{"terms":[{"template":""}]}`,
		`{"terms":[{"id":"x","template":"a"},{"id":"x","template":"b"}]}`,
		`{"terms":[{"template":"{{.FirstName}"}]}`,
		`not json`,
	}
	for _, body := range cases {
		if _, err := ParseConfig([]byte(body)); err == nil {
			t.Fatalf("expected error for %q", body)
		}
	}
}

func TestClampAndWrap(t *testing.T) {
	c := Builtin()
	if i, ch := c.ClampIndex(99); i != 1 || !ch {
		t.Fatalf("clamp high: i=%d ch=%v", i, ch)
	}
	if i, ch := c.ClampIndex(-3); i != 0 || !ch {
		t.Fatalf("clamp low: i=%d ch=%v", i, ch)
	}
	if i, ch := c.ClampIndex(1); i != 1 || ch {
		t.Fatalf("clamp ok: i=%d ch=%v", i, ch)
	}

	next, ok := c.WrapIndex(0, 1)
	if !ok || next != 1 {
		t.Fatalf("wrap forward: next=%d ok=%v", next, ok)
	}
	next, ok = c.WrapIndex(1, 1)
	if !ok || next != 0 {
		t.Fatalf("wrap last→first: next=%d ok=%v", next, ok)
	}
	next, ok = c.WrapIndex(0, -1)
	if !ok || next != 1 {
		t.Fatalf("wrap first→last: next=%d ok=%v", next, ok)
	}

	one, err := ParseConfig([]byte(`{"terms":[{"template":"{{.Name}}"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := one.WrapIndex(0, 1); ok {
		t.Fatal("single term wrap should not ok")
	}
}

func TestDisplayIndex(t *testing.T) {
	c := Builtin()
	if got := c.DisplayIndex(0); got != "T1/2" {
		t.Fatalf("got %q", got)
	}
	if got := c.DisplayIndex(1); got != "T2/2" {
		t.Fatalf("got %q", got)
	}
}

func TestLoadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "search_terms.json")
	body := `{"terms":[{"id":"x","template":"{{.Party}} mp"}]}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := LoadFile(path)
	if err != nil || !c.FromFile {
		t.Fatalf("err=%v fromFile=%v", err, c.FromFile)
	}
	got, err := c.Render(0, sampleMP())
	if err != nil {
		t.Fatal(err)
	}
	if got != "IND mp" {
		t.Fatalf("got %q", got)
	}
}
