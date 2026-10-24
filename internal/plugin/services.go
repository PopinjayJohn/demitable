package plugin

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Service is one provided service.
type Service struct {
	ID       string
	Version  string
	Provider string
	Impl     any
}

// ServiceRegistry holds services.
type ServiceRegistry struct {
	mu       sync.RWMutex
	services map[string][]Service
}

// NewServiceRegistry makes an empty registry.
func NewServiceRegistry() *ServiceRegistry {
	return &ServiceRegistry{services: map[string][]Service{}}
}

// Provide registers a service.
func (r *ServiceRegistry) Provide(s Service) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.services[s.ID] = append(r.services[s.ID], s)
}

// Resolve finds a service by id and version range.
func (r *ServiceRegistry) Resolve(id, versionRange string) (Service, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, s := range r.services[id] {
		if matchVersion(s.Version, versionRange) {
			return s, true
		}
	}
	return Service{}, false
}

// List returns all services.
func (r *ServiceRegistry) List() []Service {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []Service
	for _, list := range r.services {
		out = append(out, list...)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// matchVersion is a tiny semver range matcher.
func matchVersion(version, rng string) bool {
	rng = strings.TrimSpace(rng)
	if rng == "" || rng == "*" {
		return true
	}
	// support ">=a <b" and exact
	parts := strings.Fields(rng)
	ok := true
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		switch {
		case strings.HasPrefix(part, ">="):
			if cmpSemver(version, strings.TrimPrefix(part, ">=")) < 0 {
				ok = false
			}
		case strings.HasPrefix(part, ">"):
			if cmpSemver(version, strings.TrimPrefix(part, ">")) <= 0 {
				ok = false
			}
		case strings.HasPrefix(part, "<="):
			if cmpSemver(version, strings.TrimPrefix(part, "<=")) > 0 {
				ok = false
			}
		case strings.HasPrefix(part, "<"):
			if cmpSemver(version, strings.TrimPrefix(part, "<")) >= 0 {
				ok = false
			}
		case strings.HasPrefix(part, "="):
			if cmpSemver(version, strings.TrimPrefix(part, "=")) != 0 {
				ok = false
			}
		default:
			if cmpSemver(version, part) != 0 {
				ok = false
			}
		}
	}
	return ok
}

func cmpSemver(a, b string) int {
	// strip leading v
	a = strings.TrimPrefix(a, "v")
	b = strings.TrimPrefix(b, "v")
	pa := strings.Split(a, ".")
	pb := strings.Split(b, ".")
	for i := 0; i < 3; i++ {
		va, vb := 0, 0
		if i < len(pa) {
			fmt.Sscan(pa[i], &va)
		}
		if i < len(pb) {
			fmt.Sscan(pb[i], &vb)
		}
		if va != vb {
			if va < vb {
				return -1
			}
			return 1
		}
	}
	return 0
}
