package executor

import "testing"

func TestOptionsDefaultUserAgent(t *testing.T) {
	if got := (Options{}).withDefaults().UserAgent; got != "scrape-kdl/1.0" {
		t.Fatalf("default User-Agent = %q, want scrape-kdl/1.0", got)
	}
	if got := (Options{UserAgent: "custom"}).withDefaults().UserAgent; got != "custom" {
		t.Fatalf("explicit User-Agent = %q, want custom", got)
	}
}
