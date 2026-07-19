package executor

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"testing"

	"golang.org/x/net/dns/dnsmessage"
)

func TestRejectNonPublicAddr(t *testing.T) {
	tests := []struct {
		addr   string
		public bool
	}{
		{addr: "93.184.216.34", public: true},
		{addr: "2606:2800:220:1:248:1893:25c8:1946", public: true},
		{addr: "192.0.0.9", public: true},
		{addr: "64:ff9b::5db8:d822", public: true},
		{addr: "2001:3::1", public: true},
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
		{addr: "100.64.0.1", public: false},
		{addr: "192.0.2.1", public: false},
		{addr: "198.18.0.1", public: false},
		{addr: "198.51.100.1", public: false},
		{addr: "203.0.113.1", public: false},
		{addr: "255.255.255.255", public: false},
		{addr: "64:ff9b:1::1", public: false},
		{addr: "100::1", public: false},
		{addr: "2001:db8::1", public: false},
		{addr: "2002:7f00:1::", public: false},
		{addr: "3fff::1", public: false},
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
		"empty.example":   {},
	})
	tests := []struct {
		target string
		reject bool
	}{
		{target: "https://public.example/"},
		{target: "https://rebound.example/", reject: true},
		{target: "https://unknown.example/", reject: true},
		{target: "https://empty.example/", reject: true},
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

func TestPublicInternetHTTPClientDoesNotDelegateTargetResolutionToProxy(t *testing.T) {
	transport, ok := NewPublicInternetHTTPClient().Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport = %T", NewPublicInternetHTTPClient().Transport)
	}
	if transport.Proxy != nil {
		t.Fatal("guarded HTTP client inherited environment proxy resolution")
	}
}

type redirectToPrivateTransport struct {
	calls int
}

func (transport *redirectToPrivateTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	transport.calls++
	if transport.calls != 1 {
		return nil, errors.New("private redirect reached the transport")
	}
	return &http.Response{
		StatusCode: http.StatusFound,
		Header:     http.Header{"Location": {"http://127.0.0.1/private"}},
		Body:       io.NopCloser(strings.NewReader("")),
		Request:    request,
	}, nil
}

func TestPublicInternetURLPolicyRejectsRedirectToPrivateBeforeSecondRequest(t *testing.T) {
	transport := &redirectToPrivateTransport{}
	policy := publicInternetURLPolicy(staticResolver{"public.example": {netip.MustParseAddr("93.184.216.34")}})
	extractor := compileHTTPRuntimeSpec(t, "https://public.example/")
	_, err := Execute(context.Background(), extractor, nil, Options{
		HTTPClient: &http.Client{Transport: transport},
		URLPolicy:  policy,
	})
	var execution *ExecutionError
	if !errors.As(err, &execution) || execution.Code != "E_URL_POLICY" {
		t.Fatalf("redirect error = %v, want E_URL_POLICY", err)
	}
	if transport.calls != 1 {
		t.Fatalf("transport calls = %d, want 1", transport.calls)
	}
}

func TestPublicInternetHTTPClientRejectsPrivateAddressFromDialTimeDNS(t *testing.T) {
	resolver := resolverReturningAddr(t, netip.MustParseAddr("127.0.0.1"))
	_, err := newPublicInternetHTTPClient(resolver).Get("http://rebind.example.com/")
	var policyErr *urlPolicyError
	if !errors.As(err, &policyErr) {
		t.Fatalf("dial-time DNS error = %v, want urlPolicyError", err)
	}
}

func resolverReturningAddr(t *testing.T, answer netip.Addr) *net.Resolver {
	t.Helper()
	server, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })
	go serveDNS(t, server, answer)
	return &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, "udp4", server.LocalAddr().String())
		},
	}
}

func serveDNS(t *testing.T, server net.PacketConn, answer netip.Addr) {
	t.Helper()
	buffer := make([]byte, 512)
	for {
		n, peer, err := server.ReadFrom(buffer)
		if err != nil {
			return
		}
		var parser dnsmessage.Parser
		header, err := parser.Start(buffer[:n])
		if err != nil {
			t.Errorf("parse DNS header: %v", err)
			return
		}
		question, err := parser.Question()
		if err != nil {
			t.Errorf("parse DNS question: %v", err)
			return
		}
		header.Response = true
		header.Authoritative = true
		builder := dnsmessage.NewBuilder(nil, header)
		if err := builder.StartQuestions(); err != nil {
			t.Errorf("start DNS questions: %v", err)
			return
		}
		if err := builder.Question(question); err != nil {
			t.Errorf("write DNS question: %v", err)
			return
		}
		if err := builder.StartAnswers(); err != nil {
			t.Errorf("start DNS answers: %v", err)
			return
		}
		if question.Type == dnsmessage.TypeA && answer.Is4() {
			if err := builder.AResource(dnsmessage.ResourceHeader{Name: question.Name, Type: dnsmessage.TypeA, Class: dnsmessage.ClassINET, TTL: 60}, dnsmessage.AResource{A: answer.As4()}); err != nil {
				t.Errorf("write DNS answer: %v", err)
				return
			}
		}
		response, err := builder.Finish()
		if err != nil {
			t.Errorf("finish DNS response: %v", err)
			return
		}
		if _, err := server.WriteTo(response, peer); err != nil {
			return
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
