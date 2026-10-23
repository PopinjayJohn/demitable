// Package config holds the demitable configuration surface.
package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
)

// Config is the parsed and validated configuration.
type Config struct {
	Vault           string
	Port            int
	NoBrowser       bool
	Expose          bool
	PublicURL       string
	TrustProxy      string
	AllowHTTPImport bool
	NoImport        bool
	LogLevel        string
	LogFormat       string
	AuditLog        string
}

// Validate checks the configuration and returns an error describing the
// first problem found.
func (c Config) Validate() error {
	if c.Port < 0 || c.Port > 65535 {
		return fmt.Errorf("config: --port %d is out of range", c.Port)
	}
	if c.PublicURL != "" {
		u, err := url.Parse(c.PublicURL)
		if err != nil {
			return fmt.Errorf("config: --public-url is not a URL: %w", err)
		}
		if !u.IsAbs() {
			return errors.New("config: --public-url must be absolute")
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return fmt.Errorf("config: --public-url scheme must be http or https, got %q", u.Scheme)
		}
		if u.Host == "" {
			return errors.New("config: --public-url must have a host")
		}
		if u.RawQuery != "" {
			return errors.New("config: --public-url must not contain a query")
		}
		if u.Fragment != "" {
			return errors.New("config: --public-url must not contain a fragment")
		}
	}
	if c.Expose && c.PublicURL == "" {
		return errors.New("config: --public-url is required under --expose")
	}
	if c.TrustProxy != "" {
		if _, _, err := net.ParseCIDR(c.TrustProxy); err != nil {
			if net.ParseIP(c.TrustProxy) == nil {
				return fmt.Errorf("config: --trust-proxy %q is not a valid IP or CIDR", c.TrustProxy)
			}
		}
	}
	return nil
}

// BindAddr returns the address the server should listen on.
func (c Config) BindAddr() string {
	host := "127.0.0.1"
	if c.Expose {
		host = "0.0.0.0"
	}
	return net.JoinHostPort(host, strconv.Itoa(c.Port))
}
