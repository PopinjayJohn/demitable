package importer

import (
	"fmt"
	"net"
	"net/url"
)

// MaxDownloadBytes caps the size of a downloaded archive.
const MaxDownloadBytes = 256 * 1024 * 1024

// MaxRedirects caps redirect hops.
const MaxRedirects = 5

// isPrivateIP reports whether ip is in a range that must never be fetched.
func isPrivateIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	if ip.IsPrivate() {
		return true
	}
	// Explicitly cover the ranges named in ADR-0031 even on older stdlib.
	v4 := ip.To4()
	if v4 != nil {
		blocks := []string{"127.0.0.0/8", "10.0.0.0/8", "172.16.0.0/12",
			"192.168.0.0/16", "169.254.0.0/16"}
		for _, b := range blocks {
			_, n, err := net.ParseCIDR(b)
			if err == nil && n.Contains(v4) {
				return true
			}
		}
	}
	return false
}

// LookupIP is the DNS resolver indirection, so tests can stub it.
var LookupIP = net.LookupIP

// resolvePublicIPs returns the public IPs for host, refusing private ones.
func resolvePublicIPs(host string) ([]net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		if isPrivateIP(ip) {
			return nil, fmt.Errorf("import: private address refused: %s", ip)
		}
		return []net.IP{ip}, nil
	}
	ips, err := LookupIP(host)
	if err != nil {
		return nil, fmt.Errorf("import: resolve %s: %w", host, err)
	}
	var out []net.IP
	for _, ip := range ips {
		if !isPrivateIP(ip) {
			out = append(out, ip)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("import: %s resolves only to private/loopback/link-local addresses", host)
	}
	return out, nil
}

// validateScheme checks the URL scheme.
func validateScheme(u *url.URL, allowHTTP bool) error {
	switch u.Scheme {
	case "https":
		return nil
	case "http":
		if allowHTTP {
			return nil
		}
		return fmt.Errorf("import: plain http refused (use --allow-http-import)")
	case "file":
		return nil
	default:
		return fmt.Errorf("import: unsupported scheme %q", u.Scheme)
	}
}

// Exported wrappers so other packages (e.g. the WASM host) can reuse the
// same SSRF posture instead of reimplementing it.
func IsPrivateIP(ip net.IP) bool { return isPrivateIP(ip) }

func ResolvePublicIPs(host string) ([]net.IP, error) { return resolvePublicIPs(host) }

func ValidateScheme(u *url.URL, allowHTTP bool) error { return validateScheme(u, allowHTTP) }
