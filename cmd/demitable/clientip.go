package main

import (
	"net"
	"net/http"
	"strings"
)

// clientIP returns the address a request came from, for rate-limiting and
// audit purposes.
//
// The rule is ADR-0031's: **forwarded headers are honoured only when the
// request came from the configured proxy.** `X-Forwarded-For` is
// attacker-controlled otherwise, and a rate limiter keyed on it is a
// rate limiter an attacker can reset by forging a header.
//
// So the answer is the socket's peer address, unless that peer is the
// trusted proxy — in which case the first entry of `X-Forwarded-For` is
// the client.
//
// The trusted proxy is configured as an IP or a CIDR. A CIDR is matched
// with net.ParseCIDR, which is what makes "the proxy sits in 10.0.0.0/8"
// expressible.
func clientIP(r *http.Request) string {
	peer := r.RemoteAddr
	host, _, err := net.SplitHostPort(peer)
	if err != nil {
		// RemoteAddr is normally host:port; if it is not, use it whole.
		host = peer
	}
	if !trustedProxyAddr(host) {
		return host
	}
	// The peer is the proxy, so the first forwarded address is the client.
	// The list is "client, proxy1, proxy2", so the first entry is the one
	// the proxy saw.
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		first, _, _ := strings.Cut(xff, ",")
		if first = strings.TrimSpace(first); first != "" {
			return first
		}
	}
	if xr := strings.TrimSpace(r.Header.Get("X-Real-IP")); xr != "" {
		return xr
	}
	return host
}

// trustProxy is the configured proxy address, and trustProxyIP the
// parsed network it belongs to. Both are empty when no proxy is
// configured, which means no forwarded header is believed.
var (
	trustProxy   string
	trustProxyIP *net.IPNet
)

func setTrustedProxy(spec string) {
	trustProxy = spec
	trustProxyIP = nil
	if spec == "" {
		return
	}
	if ip := net.ParseIP(spec); ip != nil {
		bits := 32
		if ip.To4() == nil {
			bits = 128
		}
		trustProxyIP = &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)}
		return
	}
	if _, network, err := net.ParseCIDR(spec); err == nil {
		trustProxyIP = network
	}
}

// trustedProxyAddr reports whether an address is the configured proxy.
//
// An empty configuration trusts nothing, which is the safe default: the
// whole point of ADR-0031's row is that a forwarded header is only
// believed when the operator has said who may send it.
func trustedProxyAddr(host string) bool {
	if trustProxy == "" || trustProxyIP == nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && trustProxyIP.Contains(ip)
}
