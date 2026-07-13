package executor

import (
	"context"
	"errors"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"testing"
	"time"
)

func TestClientWithURLPolicyPreservesCallerClient(t *testing.T) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	originalRedirectCalls := 0
	original := &http.Client{
		Transport: http.DefaultTransport,
		Jar:       jar,
		Timeout:   2 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			originalRedirectCalls++
			return nil
		},
	}
	if got := clientWithURLPolicy(original, nil); got != original {
		t.Fatal("nil policy cloned the HTTP client")
	}

	policyCalls := 0
	wrapped := clientWithURLPolicy(original, func(context.Context, *url.URL) error {
		policyCalls++
		return nil
	})
	if wrapped == original {
		t.Fatal("non-nil policy mutated the caller-owned HTTP client")
	}
	if wrapped.Transport != original.Transport || wrapped.Jar != original.Jar || wrapped.Timeout != original.Timeout {
		t.Fatalf("clone fields differ: transport=%v jar=%v timeout=%v", wrapped.Transport == original.Transport, wrapped.Jar == original.Jar, wrapped.Timeout)
	}
	request, err := http.NewRequest(http.MethodGet, "https://example.invalid/redirect", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := original.CheckRedirect(request, nil); err != nil {
		t.Fatal(err)
	}
	if policyCalls != 0 || originalRedirectCalls != 1 {
		t.Fatalf("original callback calls: policy=%d redirect=%d", policyCalls, originalRedirectCalls)
	}
	if err := wrapped.CheckRedirect(request, nil); err != nil {
		t.Fatal(err)
	}
	if policyCalls != 1 || originalRedirectCalls != 2 {
		t.Fatalf("wrapped callback calls: policy=%d redirect=%d", policyCalls, originalRedirectCalls)
	}
}

func TestClientWithURLPolicyRejectsBeforeCallerRedirect(t *testing.T) {
	redirectCalls := 0
	original := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		redirectCalls++
		return nil
	}}
	blocked := errors.New("blocked")
	wrapped := clientWithURLPolicy(original, func(context.Context, *url.URL) error { return blocked })
	request, err := http.NewRequest(http.MethodGet, "https://blocked.invalid/", nil)
	if err != nil {
		t.Fatal(err)
	}
	err = wrapped.CheckRedirect(request, nil)
	var policy *urlPolicyError
	if !errors.As(err, &policy) || !errors.Is(err, blocked) || policy.url != request.URL.String() {
		t.Fatalf("error = %#v", err)
	}
	if redirectCalls != 0 {
		t.Fatalf("caller redirect callback invoked %d times", redirectCalls)
	}
	if err := original.CheckRedirect(request, nil); err != nil || redirectCalls != 1 {
		t.Fatalf("caller client changed: error=%v calls=%d", err, redirectCalls)
	}
}
