package db

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeTemp(t *testing.T, name, contents string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	return path
}

func TestValidateMPJSONFile_ValidArray(t *testing.T) {
	path := writeTemp(t, "mps.json", `[
  {"honorific":"Ms","firstName":"Jane","surnname":"Doe","domains":[]}
]`)
	status := ValidateMPJSONFile(path)
	if !status.Valid() {
		t.Fatalf("expected valid, got %v", status.Err)
	}
	if len(status.MPs) != 1 {
		t.Fatalf("expected 1 MP, got %d", len(status.MPs))
	}
	if status.MPs[0].FirstName != "Jane" {
		t.Errorf("FirstName=%q", status.MPs[0].FirstName)
	}
	if got := status.LogMessage(); got != "INFO mp_data.json valid" {
		t.Errorf("LogMessage=%q", got)
	}
}

func TestValidateMPJSONFile_EmptyArray(t *testing.T) {
	path := writeTemp(t, "empty.json", `[]`)
	status := ValidateMPJSONFile(path)
	if !status.Valid() {
		t.Fatalf("expected valid empty array, got %v", status.Err)
	}
	if len(status.MPs) != 0 {
		t.Fatalf("expected 0 MPs, got %d", len(status.MPs))
	}
}

func TestValidateMPJSONFile_SyntaxError(t *testing.T) {
	path := writeTemp(t, "bad.json", "[\n{\"firstName\":\n")
	status := ValidateMPJSONFile(path)
	if status.Valid() {
		t.Fatal("expected invalid for truncated JSON")
	}
	if status.Err == nil || status.Err.Error() == "" {
		t.Fatal("expected non-empty error reason")
	}
	if status.Line < 1 {
		t.Fatalf("expected line >= 1, got %d", status.Line)
	}
	msg := status.LogMessage()
	if !strings.HasPrefix(msg, "ERROR mp_data.json invalid path=") {
		t.Errorf("LogMessage=%q", msg)
	}
	if !strings.Contains(msg, "size=") || !strings.Contains(msg, "line=") {
		t.Errorf("LogMessage missing size/line: %q", msg)
	}
}

func TestValidateMPJSONFile_WrongRoot(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"object", `{"firstName":"x"}`},
		{"null", `null`},
		{"scalar", `"hello"`},
		{"number", `42`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeTemp(t, "root.json", tc.body)
			status := ValidateMPJSONFile(path)
			if status.Valid() {
				t.Fatalf("expected invalid for root %s", tc.name)
			}
			if !strings.Contains(status.Err.Error(), "root must be a JSON array") &&
				!strings.Contains(status.Err.Error(), "array") {
				// null/object go through our root check; keep reason useful
				if status.Err.Error() == "" {
					t.Fatal("empty error")
				}
			}
		})
	}
}

func TestValidateMPJSONFile_TrailingGarbage(t *testing.T) {
	path := writeTemp(t, "trail.json", "[]\n,\n{\"x\":1}\n")
	status := ValidateMPJSONFile(path)
	if status.Valid() {
		t.Fatal("expected invalid for trailing garbage")
	}
	if !strings.Contains(status.Err.Error(), "trailing garbage") {
		t.Errorf("error=%v", status.Err)
	}
	if status.Line != 2 {
		t.Errorf("Line=%d want 2 (comma after closed array)", status.Line)
	}
	msg := status.LogMessage()
	if !strings.Contains(msg, "line=2") {
		t.Errorf("LogMessage=%q", msg)
	}
}

func TestValidateMPJSONFile_TrailingGarbage_AfterArrayClose(t *testing.T) {
	// Mimics a premature `]` then more objects (as in a broken mp_data.json).
	body := "[\n  {\"firstName\":\"A\"}\n]\n,\n  {\"firstName\":\"B\"}\n"
	path := writeTemp(t, "premature_close.json", body)
	status := ValidateMPJSONFile(path)
	if status.Valid() {
		t.Fatal("expected invalid")
	}
	if status.Line != 4 {
		t.Errorf("Line=%d want 4 (comma after closed array)", status.Line)
	}
	if !strings.Contains(status.LogMessage(), "line=4") {
		t.Errorf("LogMessage=%q", status.LogMessage())
	}
}

func TestValidateMPJSONFile_WrongRoot_ReportsLine(t *testing.T) {
	path := writeTemp(t, "root.json", "  \nnull\n")
	status := ValidateMPJSONFile(path)
	if status.Valid() {
		t.Fatal("expected invalid")
	}
	if status.Line != 2 {
		t.Errorf("Line=%d want 2", status.Line)
	}
}

func TestValidateMPJSONFile_NestedDomainLastChecked(t *testing.T) {
	body := `[{
  "firstName":"Ada",
  "surnname":"Lovelace",
  "domains":[{
    "hostname":"example.com",
    "expiry":"2030-01-01",
    "expired":false,
    "lastChecked":"2026-03-01T12:00:00Z"
  }]
}]`
	path := writeTemp(t, "nested.json", body)
	status := ValidateMPJSONFile(path)
	if !status.Valid() {
		t.Fatalf("expected valid, got %v", status.Err)
	}
	if len(status.MPs) != 1 || len(status.MPs[0].Domains) != 1 {
		t.Fatalf("unexpected structure: %+v", status.MPs)
	}
	d := status.MPs[0].Domains[0]
	if d.Hostname != "example.com" {
		t.Errorf("hostname=%q", d.Hostname)
	}
	if d.LastChecked.IsZero() {
		t.Error("expected lastChecked parsed")
	}
	if !d.LastChecked.Equal(time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)) {
		t.Errorf("lastChecked=%v", d.LastChecked)
	}
}

func TestValidateMPJSONFile_PathAndSize(t *testing.T) {
	contents := `[{"nope"`
	path := writeTemp(t, "sized.json", contents)
	status := ValidateMPJSONFile(path)
	if status.Valid() {
		t.Fatal("expected invalid")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	if status.Path != abs {
		t.Errorf("Path=%q want %q", status.Path, abs)
	}
	wantBytes := int64(len(contents))
	if status.SizeBytes != wantBytes {
		t.Errorf("SizeBytes=%d want %d", status.SizeBytes, wantBytes)
	}
	wantKB := float64(wantBytes) / 1024.0
	if math.Abs(status.SizeKB()-wantKB) > 0.0001 {
		t.Errorf("SizeKB=%v want %v", status.SizeKB(), wantKB)
	}
	msg := status.LogMessage()
	if !strings.Contains(msg, abs) {
		t.Errorf("log missing abs path: %q", msg)
	}
	if !strings.Contains(msg, "size=") {
		t.Errorf("log missing size: %q", msg)
	}
}

func TestValidateMPJSONFile_MissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.json")
	status := ValidateMPJSONFile(path)
	if status.Valid() {
		t.Fatal("expected invalid for missing file")
	}
	abs, _ := filepath.Abs(path)
	if status.Path != abs {
		t.Errorf("Path=%q want %q", status.Path, abs)
	}
	if status.SizeBytes != 0 {
		t.Errorf("SizeBytes=%d want 0", status.SizeBytes)
	}
	msg := status.LogMessage()
	if !strings.Contains(msg, abs) || !strings.Contains(msg, "size=0.0KB") {
		t.Errorf("LogMessage=%q", msg)
	}
}

func TestDecodeMpsStrict_Direct(t *testing.T) {
	_, _, err := decodeMpsStrict([]byte(`[]`))
	if err != nil {
		t.Fatal(err)
	}
}

func TestOffsetToLineCol(t *testing.T) {
	buf := []byte("a\nbb\nccc")
	// offset 0 -> line 1 col 1
	if p := offsetToLineCol(buf, 0); p.line != 1 || p.col != 1 {
		t.Errorf("offset 0 -> %+v", p)
	}
	// offset 2 is start of "bb" (after 'a\n')
	if p := offsetToLineCol(buf, 2); p.line != 2 || p.col != 1 {
		t.Errorf("offset 2 -> %+v", p)
	}
}
