package executor

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"testing"
)

func TestRejectNonPublicAddr(t *testing.T) {
	tests := []struct {
		addr   string
		public bool
	}{
		{addr: "93.184.216.34", public: true},
		{addr: "2606:2800:220:1:248:1893:25c8:1946", public: true},
		{addr: "127.0.0.1", public: false},
		{addr: "::1", public: false},
		{addr: "10.0.0.8", public: false},
		{addr: "172.16.0.1", public: false},
		{addr: "192.168.1.1", public: false},
		{addr: "169.254.169.254", public: false},
		{addr: "0.0.0.0", public: false},
		{addr: "::", public: false},
		{addr: "fd00::1", public: false},
		{addr: "fe80::1", public: false},
		{addr: "224.0.0.1", public: false},
		{addr: "::ffff:127.0.0.1", public: false},
		{addr: "::ffff:192.168.1.1", public: false},
	}
	for _, tt := range tests {
		err := rejectNonPublicAddr(netip.MustParseAddr(tt.addr))
		if (err == nil) != tt.public {
			t.Errorf("rejectNonPublicAddr(%s) = %v, want public=%v", tt.addr, err, tt.public)
		}
	}
}

func TestPublicInternetURLPolicy(t *testing.T) {
	policy := PublicInternetURLPolicy()
	reject := []string{
		"ftp://example.com/",
		"http://user:pass@example.com/",
		"http://127.0.0.1/",
		"http://[::1]:8080/",
		"http://169.254.169.254/latest/meta-data/",
		"http://192.168.0.10/admin",
		"http://localhost/",
	}
	for _, target := range reject {
		parsed, err := url.Parse(target)
		if err != nil {
			t.Fatal(err)
		}
		if policy(context.Background(), parsed) == nil {
			t.Errorf("policy accepted %s", target)
		}
	}
}

func TestPublicInternetHTTPClientBlocksDial(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer server.Close()
	_, err := NewPublicInternetHTTPClient().Get(server.URL)
	var policyErr *urlPolicyError
	if !errors.As(err, &policyErr) {
		t.Fatalf("dial to loopback = %v, want urlPolicyError", err)
	}
}

func TestFetchDocumentMapsDialRejectionToURLPolicy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer server.Close()
	options := Options{HTTPClient: NewPublicInternetHTTPClient()}.withDefaults()
	_, err := fetchDocument(context.Background(), server.URL, options)
	var execution *ExecutionError
	if !errors.As(err, &execution) || execution.Code != "E_URL_POLICY" {
		t.Fatalf("fetch error = %v, want E_URL_POLICY", err)
	}
	if !strings.Contains(execution.Message, "not a public internet address") {
		t.Fatalf("fetch error message = %q", execution.Message)
	}
}
