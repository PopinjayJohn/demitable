package importer

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// FetchArchive downloads url to a temporary file, applying SSRF defences.
// It returns the temp file path; the caller removes it.
func FetchArchive(ctx context.Context, rawURL string, allowHTTP bool) (string, error) {
	current := rawURL
	for hop := 0; hop <= MaxRedirects; hop++ {
		u, err := url.Parse(current)
		if err != nil {
			return "", fmt.Errorf("import: bad url: %w", err)
		}
		if !u.IsAbs() && u.Scheme != "file" {
			return "", fmt.Errorf("import: url must be absolute")
		}
		if err := validateScheme(u, allowHTTP); err != nil {
			return "", err
		}

		// file:// handling (no network, no SSRF).
		if u.Scheme == "file" {
			p := u.Path
			if p == "" {
				p = rawURL[len("file://"):]
			}
			// copy to temp file
			src, err := os.Open(p)
			if err != nil {
				return "", fmt.Errorf("import: open file: %w", err)
			}
			defer src.Close()
			tmp, err := os.CreateTemp("", "demitable-archive-*")
			if err != nil {
				return "", err
			}
			if _, err := io.Copy(tmp, src); err != nil {
				tmp.Close()
				os.Remove(tmp.Name())
				return "", err
			}
			tmp.Close()
			return tmp.Name(), nil
		}

		host := u.Hostname()
		// Resolve once.
		ips, err := resolvePublicIPs(host)
		if err != nil {
			return "", err
		}
		pinned := ips[0]

		// Re-resolve to detect rebinding: the second resolution must still
		// contain the pinned address and must not introduce a private one.
		ips2, err := LookupIP(host)
		if err == nil {
			foundPinned := false
			for _, ip := range ips2 {
				if isPrivateIP(ip) {
					return "", fmt.Errorf("import: dns rebinding detected: private address %s", ip)
				}
				if ip.Equal(pinned) {
					foundPinned = true
				}
			}
			if !foundPinned {
				return "", fmt.Errorf("import: dns rebinding detected: address changed")
			}
		}

		port := u.Port()
		if port == "" {
			if u.Scheme == "https" {
				port = "443"
			} else {
				port = "80"
			}
		}

		tr := &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				d := &net.Dialer{Timeout: 30 * time.Second}
				return d.DialContext(ctx, network, net.JoinHostPort(pinned.String(), port))
			},
			TLSClientConfig:       &tls.Config{ServerName: host},
			ResponseHeaderTimeout: 30 * time.Second,
		}
		client := &http.Client{
			Transport: tr,
			Timeout:   60 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, current, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("User-Agent", "demitable-import/1")
		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("import: fetch: %w", err)
		}

		ct := resp.Header.Get("Content-Type")
		if !acceptableContentType(ct) {
			resp.Body.Close()
			return "", fmt.Errorf("import: unexpected content type %q", ct)
		}

		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			loc := resp.Header.Get("Location")
			resp.Body.Close()
			if loc == "" {
				return "", fmt.Errorf("import: redirect without location")
			}
			next, err := u.Parse(loc)
			if err != nil {
				return "", fmt.Errorf("import: bad redirect: %w", err)
			}
			current = next.String()
			continue
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return "", fmt.Errorf("import: http status %d", resp.StatusCode)
		}

		tmp, err := os.CreateTemp("", "demitable-archive-*")
		if err != nil {
			resp.Body.Close()
			return "", err
		}
		n, err := io.Copy(tmp, io.LimitReader(resp.Body, MaxDownloadBytes+1))
		resp.Body.Close()
		tmp.Close()
		if err != nil {
			os.Remove(tmp.Name())
			return "", err
		}
		if n > MaxDownloadBytes {
			os.Remove(tmp.Name())
			return "", fmt.Errorf("import: download exceeds %d bytes", MaxDownloadBytes)
		}
		return tmp.Name(), nil
	}
	return "", fmt.Errorf("import: too many redirects")
}

func acceptableContentType(ct string) bool {
	ct = strings.ToLower(strings.TrimSpace(ct))
	if ct == "" {
		return true
	}
	if strings.Contains(ct, "zip") {
		return true
	}
	if strings.Contains(ct, "octet-stream") {
		return true
	}
	return false
}
