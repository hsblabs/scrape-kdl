package rodadapter

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-rod/rod"
	scrapekdl "github.com/hsblabs/scrape-kdl"
)

func TestAdapterSatisfiesBrowserAdapter(t *testing.T) {
	var _ scrapekdl.BrowserAdapter = (*Adapter)(nil)
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
