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

func TestEnforceURLPolicyMapsValidationAndRejection(t *testing.T) {
	if err := enforceURLPolicy(context.Background(), "https://example.invalid/", nil); err != nil {
		t.Fatalf("nil policy error = %v", err)
	}
	policyCalls := 0
	allow := func(_ context.Context, target *url.URL) error {
		policyCalls++
		if target.Host != "example.invalid" {
			t.Fatalf("target = %s", target)
		}
		return nil
	}
	if err := enforceURLPolicy(context.Background(), "https://example.invalid/", allow); err != nil || policyCalls != 1 {
		t.Fatalf("allow error = %v, calls = %d", err, policyCalls)
	}
	if err := enforceURLPolicy(context.Background(), "https://example.invalid/%", allow); err == nil {
		t.Fatal("invalid URL passed policy enforcement")
	} else {
		var execution *ExecutionError
		if !errors.As(err, &execution) || execution.Code != "E_URL_INVALID" || policyCalls != 1 {
			t.Fatalf("invalid URL error = %#v, calls = %d", err, policyCalls)
		}
	}

	blocked := errors.New("blocked")
	err := enforceURLPolicy(context.Background(), "https://blocked.invalid/", func(context.Context, *url.URL) error { return blocked })
	var execution *ExecutionError
	if !errors.As(err, &execution) || execution.Code != "E_URL_POLICY" || !errors.Is(err, blocked) {
		t.Fatalf("rejection error = %#v", err)
	}
}

func TestConvertHTTPPolicyErrorDistinguishesWrappedErrors(t *testing.T) {
	blocked := errors.New("blocked")
	policy := &urlPolicyError{url: "https://blocked.invalid/", cause: blocked}
	for _, tt := range []struct {
		name string
		err  error
	}{
		{name: "direct", err: policy},
		{name: "HTTP client URL wrapper", err: &url.Error{Op: "Get", URL: policy.url, Err: policy}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			converted := convertHTTPPolicyError(tt.err)
			var execution *ExecutionError
			if !errors.As(converted, &execution) || execution.Code != "E_URL_POLICY" || !errors.Is(converted, blocked) {
				t.Fatalf("converted error = %#v", converted)
			}
		})
	}
	if converted := convertHTTPPolicyError(errors.New("transport failed")); converted != nil {
		t.Fatalf("unrelated error converted = %#v", converted)
	}
}
