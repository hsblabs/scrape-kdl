package executor

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"syscall"
)

// PublicInternetURLPolicy rejects targets that are not plain public-internet
// HTTP(S) URLs: other schemes, userinfo, and hosts whose literal or resolved
// addresses are loopback, private, link-local, multicast, or unspecified.
// Resolution here is advisory; pair it with NewPublicInternetHTTPClient,
// which re-checks the address actually dialed, to cover DNS rebinding.
func PublicInternetURLPolicy() URLPolicy {
	return publicInternetURLPolicy(net.DefaultResolver)
}

type hostResolver interface {
	LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error)
}

// The IANA special-purpose registries contain public exceptions nested inside
// broader non-public prefixes, so exceptions must be checked first.
var globallyReachableSpecialPrefixes = []netip.Prefix{
	netip.MustParsePrefix("192.0.0.9/32"),
	netip.MustParsePrefix("192.0.0.10/32"),
	netip.MustParsePrefix("2001:1::1/128"),
	netip.MustParsePrefix("2001:1::2/128"),
	netip.MustParsePrefix("2001:1::3/128"),
	netip.MustParsePrefix("2001:3::/32"),
	netip.MustParsePrefix("2001:4:112::/48"),
	netip.MustParsePrefix("2001:20::/28"),
	netip.MustParsePrefix("2001:30::/28"),
}

var nonPublicSpecialPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("169.254.0.0/16"),
	netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"),
	netip.MustParsePrefix("192.168.0.0/16"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("64:ff9b:1::/48"),
	netip.MustParsePrefix("100::/64"),
	netip.MustParsePrefix("100:0:0:1::/64"),
	netip.MustParsePrefix("2001::/23"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("3fff::/20"),
	netip.MustParsePrefix("5f00::/16"),
	netip.MustParsePrefix("fc00::/7"),
}

func publicInternetURLPolicy(resolver hostResolver) URLPolicy {
	return func(ctx context.Context, target *url.URL) error {
		host, err := classifyPublicTarget(target)
		if err != nil || host == "" {
			return err
		}
		addrs, err := resolver.LookupNetIP(ctx, "ip", host)
		if err != nil {
			return fmt.Errorf("resolve host %q: %w", host, err)
		}
		if len(addrs) == 0 {
			return fmt.Errorf("resolve host %q: no addresses", host)
		}
		for _, addr := range addrs {
			if err := rejectNonPublicAddr(addr); err != nil {
				return err
			}
		}
		return nil
	}
}

// classifyPublicTarget performs every check that needs no resolution. It
// returns a non-empty hostname when only DNS resolution remains, and an
// empty hostname when the target is already fully vetted.
func classifyPublicTarget(target *url.URL) (string, error) {
	if target.Scheme != "http" && target.Scheme != "https" {
		return "", fmt.Errorf("scheme %q is not allowed", target.Scheme)
	}
	if target.User != nil {
		return "", errors.New("userinfo URLs are not allowed")
	}
	host := target.Hostname()
	if host == "" {
		return "", errors.New("URL has no host")
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		return "", rejectNonPublicAddr(addr)
	}
	return host, nil
}

// NewPublicInternetHTTPClient returns an HTTP client whose dialer rejects
// non-public addresses at connection time, after DNS resolution.
func NewPublicInternetHTTPClient() *http.Client {
	return newPublicInternetHTTPClient(net.DefaultResolver)
}

func newPublicInternetHTTPClient(resolver *net.Resolver) *http.Client {
	dialer := &net.Dialer{
		Resolver: resolver,
		Control: func(network, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			addr, err := netip.ParseAddr(host)
			if err != nil {
				return err
			}
			if err := rejectNonPublicAddr(addr); err != nil {
				return &urlPolicyError{url: address, cause: err}
			}
			return nil
		},
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = dialer.DialContext
	return &http.Client{Transport: transport}
}

func rejectNonPublicAddr(addr netip.Addr) error {
	unmapped := addr.Unmap()
	for _, prefix := range globallyReachableSpecialPrefixes {
		if prefix.Contains(unmapped) {
			return nil
		}
	}
	for _, prefix := range nonPublicSpecialPrefixes {
		if prefix.Contains(unmapped) {
			return fmt.Errorf("address %s is not a public internet address", unmapped)
		}
	}
	if !unmapped.IsGlobalUnicast() || unmapped.IsLoopback() || unmapped.IsPrivate() || unmapped.IsLinkLocalUnicast() ||
		unmapped.IsMulticast() || unmapped.IsUnspecified() {
		return fmt.Errorf("address %s is not a public internet address", unmapped)
	}
	return nil
}
