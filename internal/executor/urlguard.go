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
	return func(ctx context.Context, target *url.URL) error {
		if target.Scheme != "http" && target.Scheme != "https" {
			return fmt.Errorf("scheme %q is not allowed", target.Scheme)
		}
		if target.User != nil {
			return errors.New("userinfo URLs are not allowed")
		}
		host := target.Hostname()
		if host == "" {
			return errors.New("URL has no host")
		}
		if addr, err := netip.ParseAddr(host); err == nil {
			return rejectNonPublicAddr(addr)
		}
		addrs, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		if err != nil {
			return fmt.Errorf("resolve host %q: %w", host, err)
		}
		for _, addr := range addrs {
			if err := rejectNonPublicAddr(addr); err != nil {
				return err
			}
		}
		return nil
	}
}

// NewPublicInternetHTTPClient returns an HTTP client whose dialer rejects
// non-public addresses at connection time, after DNS resolution.
func NewPublicInternetHTTPClient() *http.Client {
	dialer := &net.Dialer{
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
	transport.DialContext = dialer.DialContext
	return &http.Client{Transport: transport}
}

func rejectNonPublicAddr(addr netip.Addr) error {
	unmapped := addr.Unmap()
	if unmapped.IsLoopback() || unmapped.IsPrivate() || unmapped.IsLinkLocalUnicast() ||
		unmapped.IsMulticast() || unmapped.IsUnspecified() {
		return fmt.Errorf("address %s is not a public internet address", unmapped)
	}
	return nil
}
