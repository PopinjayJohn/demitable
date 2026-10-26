package vault

import (
	"sync"

	"github.com/PopinjayJohn/demitable/internal/pack"
)

var (
	packMu    sync.RWMutex
	packReg   = map[string]*pack.Pack{}
	packTried = map[string]bool{}
)

// RegisterPack records a pack for a system id. A nil pack removes the entry.
func RegisterPack(system string, p *pack.Pack) {
	system = trim(system)
	packMu.Lock()
	defer packMu.Unlock()
	if p == nil {
		delete(packReg, system)
		return
	}
	packReg[system] = p
	packTried[system] = true
}

// PackFor returns the pack for a system, loading it from the embedded set
// on first use. A missing pack yields nil.
func PackFor(system string) *pack.Pack {
	system = trim(system)
	packMu.RLock()
	if p, ok := packReg[system]; ok {
		packMu.RUnlock()
		return p
	}
	tried := packTried[system]
	packMu.RUnlock()
	if tried {
		return nil
	}
	p, _ := pack.Load(system)
	packMu.Lock()
	packTried[system] = true
	if p != nil {
		packReg[system] = p
	}
	packMu.Unlock()
	return p
}

// SetPackFor is an alias for RegisterPack.
func SetPackFor(system string, p *pack.Pack) { RegisterPack(system, p) }

func trim(s string) string {
	// local trim to avoid import churn
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\n' || s[0] == '\r') {
		s = s[1:]
	}
	for len(s) > 0 {
		c := s[len(s)-1]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			s = s[:len(s)-1]
		} else {
			break
		}
	}
	return s
}
