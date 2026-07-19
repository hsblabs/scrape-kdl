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

func TestClassifyPublicTarget(t *testing.T) {
	tests := []struct {
		target string
		host   string
		reject bool
	}{
		{target: "https://example.com/page", host: "example.com"},
		{target: "http://example.com:8080/", host: "example.com"},
		{target: "https://93.184.216.34/", host: ""},
		{target: "ftp://example.com/", reject: true},
		{target: "http://user:pass@example.com/", reject: true},
		{target: "http://127.0.0.1/", reject: true},
		{target: "http://[::1]:8080/", reject: true},
		{target: "http://169.254.169.254/latest/meta-data/", reject: true},
		{target: "http://192.168.0.10/admin", reject: true},
	}
	for _, tt := range tests {
		parsed, err := url.Parse(tt.target)
		if err != nil {
			t.Fatal(err)
		}
		host, err := classifyPublicTarget(parsed)
		if (err != nil) != tt.reject || host != tt.host {
			t.Errorf("classifyPublicTarget(%s) = %q, %v; want host %q, reject %v", tt.target, host, err, tt.host, tt.reject)
		}
	}
}

type staticResolver map[string][]netip.Addr

func (r staticResolver) LookupNetIP(_ context.Context, _ string, host string) ([]netip.Addr, error) {
	addrs, ok := r[host]
	if !ok {
		return nil, errors.New("no such host")
	}
	return addrs, nil
}

func TestPublicInternetURLPolicyResolvedHosts(t *testing.T) {
	policy := publicInternetURLPolicy(staticResolver{
		"public.example":  {netip.MustParseAddr("93.184.216.34")},
		"rebound.example": {netip.MustParseAddr("93.184.216.34"), netip.MustParseAddr("10.0.0.8")},
	})
	tests := []struct {
		target string
		reject bool
	}{
		{target: "https://public.example/"},
		{target: "https://rebound.example/", reject: true},
		{target: "https://unknown.example/", reject: true},
	}
	for _, tt := range tests {
		parsed, err := url.Parse(tt.target)
		if err != nil {
			t.Fatal(err)
		}
		if got := policy(context.Background(), parsed); (got != nil) != tt.reject {
			t.Errorf("policy(%s) = %v, want reject %v", tt.target, got, tt.reject)
		}
	}
}

func TestPublicInternetURLPolicyDefaultResolver(t *testing.T) {
	parsed, err := url.Parse("http://localhost/")
	if err != nil {
		t.Fatal(err)
	}
	if PublicInternetURLPolicy()(context.Background(), parsed) == nil {
		t.Error("policy accepted http://localhost/")
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
