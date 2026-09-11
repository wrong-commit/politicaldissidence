package domainlookups

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseConfig_MultiTLD(t *testing.T) {
	c, err := ParseConfig([]byte(`{
  "com.au": ["godaddy"],
  "com": ["godaddy", "namecheap"]
}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := c.SourcesForHostname("example.com.au"); !reflect.DeepEqual(got, []string{"godaddy"}) {
		t.Fatalf("com.au: got %v", got)
	}
	if got := c.SourcesForHostname("www.Example.COM.AU"); !reflect.DeepEqual(got, []string{"godaddy"}) {
		t.Fatalf("www com.au: got %v", got)
	}
	if got := c.SourcesForHostname("example.com"); !reflect.DeepEqual(got, []string{"godaddy", "namecheap"}) {
		t.Fatalf("com: got %v", got)
	}
	if got := c.SourcesForHostname("foo.org"); got != nil {
		t.Fatalf("no match: got %v", got)
	}
}

func TestParseConfig_LongestSuffix(t *testing.T) {
	c, err := ParseConfig([]byte(`{
  "au": ["x"],
  "com.au": ["godaddy"]
}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := c.SourcesForHostname("a.com.au"); !reflect.DeepEqual(got, []string{"godaddy"}) {
		t.Fatalf("want godaddy, got %v", got)
	}
}

func TestParseConfig_Invalid(t *testing.T) {
	if _, err := ParseConfig([]byte(`[]`)); err == nil {
		t.Fatal("expected error for non-object")
	}
	if _, err := ParseConfig([]byte(`{"com": []}`)); err == nil {
		t.Fatal("expected error for empty sources")
	}
}

func TestLoadFile_Shipped(t *testing.T) {
	path := filepath.Join("..", DefaultConfigPath)
	if _, err := os.Stat(path); err != nil {
		t.Skip("shipped domain_lookups.json not found at", path)
	}
	c, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := c.SourcesForHostname("x.com")
	if !reflect.DeepEqual(got, []string{"godaddy", "namecheap"}) {
		t.Fatalf("shipped com sources: %v", got)
	}
	if !reflect.DeepEqual(c.SourcesForHostname("x.com.au"), []string{"godaddy"}) {
		t.Fatalf("shipped com.au: %v", c.SourcesForHostname("x.com.au"))
	}
}
