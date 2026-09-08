package searching

import "testing"

func TestEngineNormalizeNextPaging(t *testing.T) {
	if got := Engine("").Normalize(); got != EngineBing {
		t.Fatalf("empty Normalize=%q want bing", got)
	}
	if got := EngineBing.Next(); got != EngineDuckDuckGo {
		t.Fatalf("Bing.Next=%q want duckduckgo", got)
	}
	if got := EngineDuckDuckGo.Next(); got != EngineBing {
		t.Fatalf("DDG.Next=%q want bing", got)
	}
	if !EngineBing.SupportsPaging() {
		t.Fatal("Bing should support paging")
	}
	if !EngineDuckDuckGo.SupportsPaging() {
		t.Fatal("DuckDuckGo should support paging")
	}
	if EngineBing.Short() != "Bing" || EngineDuckDuckGo.Short() != "DDG" {
		t.Fatalf("Short labels: Bing=%q DDG=%q", EngineBing.Short(), EngineDuckDuckGo.Short())
	}
}
