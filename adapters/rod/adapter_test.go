package rodadapter

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	scrapekdl "github.com/hsblabs/scrape-kdl"
)

func TestAdapterSatisfiesBrowserAdapter(t *testing.T) {
	var _ scrapekdl.BrowserAdapter = (*Adapter)(nil)
}

func TestResolveKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want input.Key
	}{
		{name: "named", key: "Enter", want: input.Enter},
		{name: "generic modifier", key: "Control", want: input.ControlLeft},
		{name: "printable", key: "a", want: input.Key('a')},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveKey(tt.key)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("resolveKey(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}

	if _, err := resolveKey("NotAKey"); err == nil {
		t.Fatal("resolveKey accepted an unsupported key")
	}
}

func TestAcquireSerializesAndReleaseIsIdempotent(t *testing.T) {
	adapter := newAdapter(&rod.Page{}, false, nil)
	release, err := adapter.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, err := adapter.Acquire(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("second acquire error = %v", err)
	}

	release()
	release()

	releaseAgain, err := adapter.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	releaseAgain()
}
