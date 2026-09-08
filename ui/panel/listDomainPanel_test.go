package panel

import (
	"strings"
	"testing"
	"time"

	"politicaldissidence/data"
)

func TestDrawListDomainPanel_LastChecked(t *testing.T) {
	checked := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	domains := []data.Domain{
		{Hostname: "a.example", Expiry: "2027-01-01", LastChecked: checked},
		{Hostname: "b.example", Expiry: "", LastChecked: time.Time{}},
		{Hostname: "c.example", Expiry: "2026-01-01", Expired: true, LastChecked: checked},
	}
	got := DrawListDomainPanel(nil, &domains)

	if !strings.Contains(got, "a.example 2027-01-01  checked 26-03-01") {
		t.Fatalf("missing last-checked row:\n%s", got)
	}
	if !strings.Contains(got, "b.example <?>  checked never") {
		t.Fatalf("missing never row:\n%s", got)
	}
	if !strings.Contains(got, "c.example 2026-01-01[!]  checked 26-03-01") {
		t.Fatalf("missing expired marker row:\n%s", got)
	}
}
