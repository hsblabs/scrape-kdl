package rodadapter

import (
	"context"
	"errors"
	"net/http"
	"reflect"
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

func TestSessionValuesPreserveDeterministicOrder(t *testing.T) {
	headers := http.Header{
		"x-order": []string{"lower-one", "lower-two"},
		"X-Order": []string{"upper"},
	}
	wantHeaders := []string{"X-Order", "upper", "x-order", "lower-one", "x-order", "lower-two"}
	for range 100 {
		if got := flattenHeaders(headers); !reflect.DeepEqual(got, wantHeaders) {
			t.Fatalf("flattened headers = %v", got)
		}
	}

	params, err := cookieParams("https://example.invalid/path", []*http.Cookie{
		nil,
		{Name: "duplicate", Value: "first"},
		{Name: "duplicate", Value: "second"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(params) != 2 || params[0].Name != "duplicate" || params[0].Value != "first" || params[1].Name != "duplicate" || params[1].Value != "second" {
		t.Fatalf("cookie parameter order = %#v", params)
	}
	if _, err := cookieParams("://invalid", []*http.Cookie{{Name: "value"}}); err == nil {
		t.Fatal("cookieParams accepted an invalid target URL")
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
