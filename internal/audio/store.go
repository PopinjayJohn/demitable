// Package audio owns the session-scoped blob store for captured session
// audio (P1). The bytes live OUTSIDE the vault, under the XDG app-data
// dir (ADR-0030): audio is bulky, undiffable, and must never sync. The
// transcript — the durable text — lives in the vault; this package owns
// only bytes.
package audio

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/PopinjayJohn/demitable/internal/store"
)

// validSessionID restricts session ids used in paths.
var validSessionID = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// Store is the single blob store for session audio. One mechanism.
type Store struct {
	Base string // <dataDir>/audio
}

// New opens (creates) the audio blob store under the XDG data dir.
func New() (*Store, error) {
	dd, err := store.DataDir()
	if err != nil {
		return nil, err
	}
	base := filepath.Join(dd, "audio")
	if err := os.MkdirAll(base, 0o700); err != nil {
		return nil, fmt.Errorf("audio: mkdir: %w", err)
	}
	return &Store{Base: base}, nil
}

// ValidSessionID reports whether id is safe to use as a path segment.
func ValidSessionID(id string) bool {
	return id != "" && validSessionID.MatchString(id)
}

// SessionDir returns the per-session directory.
func (s *Store) SessionDir(sessionID string) string {
	return filepath.Join(s.Base, sessionID)
}

// FinalPath is the assembled blob path.
func (s *Store) FinalPath(sessionID string) string {
	return filepath.Join(s.SessionDir(sessionID), "audio.bin")
}

// ChunkDir is where per-chunk uploads land.
func (s *Store) ChunkDir(sessionID string) string {
	return filepath.Join(s.SessionDir(sessionID), "chunks")
}

// WriteChunk stores one chunk under a server-generated name. index must
// be >= 0. The filename is server-generated (F0b discipline); the caller's
// name is never used.
func (s *Store) WriteChunk(sessionID string, index int, data []byte) error {
	if !ValidSessionID(sessionID) {
		return fmt.Errorf("audio: bad session id")
	}
	if index < 0 {
		return fmt.Errorf("audio: bad chunk index")
	}
	dir := s.ChunkDir(sessionID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	// Server-generated basename: chunk-<index>.bin (no caller input).
	name := fmt.Sprintf("chunk-%06d.bin", index)
	full := filepath.Join(dir, name)
	// no-overwrite: a retry of the same chunk replaces? Resumable retries
	// should be idempotent for the same index, so allow overwrite of an
	// identical chunk index (same server-generated name). This is not a
	// collision with a different file.
	return os.WriteFile(full, data, 0o600)
}

// Assemble concatenates chunks 0..total-1 into the final blob. Missing
// chunks make it an error. If total <= 0, assembles all chunks found in
// index order.
func (s *Store) Assemble(sessionID string, total int) error {
	if !ValidSessionID(sessionID) {
		return fmt.Errorf("audio: bad session id")
	}
	chunks, err := s.listChunks(sessionID)
	if err != nil {
		return err
	}
	if len(chunks) == 0 {
		return fmt.Errorf("audio: no chunks")
	}
	if total > 0 && len(chunks) < total {
		return fmt.Errorf("audio: missing chunks: have %d want %d", len(chunks), total)
	}
	// determine count
	n := len(chunks)
	if total > 0 {
		n = total
	}
	var out []byte
	for i := 0; i < n; i++ {
		p := filepath.Join(s.ChunkDir(sessionID), fmt.Sprintf("chunk-%06d.bin", i))
		data, err := os.ReadFile(p)
		if err != nil {
			return fmt.Errorf("audio: missing chunk %d: %w", i, err)
		}
		out = append(out, data...)
	}
	if err := os.MkdirAll(s.SessionDir(sessionID), 0o700); err != nil {
		return err
	}
	return os.WriteFile(s.FinalPath(sessionID), out, 0o600)
}

// listChunks returns sorted chunk indices present.
func (s *Store) listChunks(sessionID string) ([]int, error) {
	dir := s.ChunkDir(sessionID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var idx []int
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, "chunk-") || !strings.HasSuffix(name, ".bin") {
			continue
		}
		mid := strings.TrimPrefix(name, "chunk-")
		mid = strings.TrimSuffix(mid, ".bin")
		i, err := strconv.Atoi(mid)
		if err != nil {
			continue
		}
		idx = append(idx, i)
	}
	sort.Ints(idx)
	return idx, nil
}

// Read returns the assembled blob, assembling on demand from chunks if the
// final blob is absent. Returns os.ErrNotExist when nothing is present.
func (s *Store) Read(sessionID string) ([]byte, error) {
	if !ValidSessionID(sessionID) {
		return nil, os.ErrNotExist
	}
	if data, err := os.ReadFile(s.FinalPath(sessionID)); err == nil {
		return data, nil
	}
	chunks, _ := s.listChunks(sessionID)
	if len(chunks) == 0 {
		return nil, os.ErrNotExist
	}
	// assemble on demand
	if err := s.Assemble(sessionID, 0); err != nil {
		return nil, err
	}
	return os.ReadFile(s.FinalPath(sessionID))
}

// Delete removes all bytes for a session.
func (s *Store) Delete(sessionID string) error {
	if !ValidSessionID(sessionID) {
		return nil
	}
	return os.RemoveAll(s.SessionDir(sessionID))
}

// Has reports whether any blob exists for the session.
func (s *Store) Has(sessionID string) bool {
	if !ValidSessionID(sessionID) {
		return false
	}
	if _, err := os.Stat(s.FinalPath(sessionID)); err == nil {
		return true
	}
	chunks, _ := s.listChunks(sessionID)
	return len(chunks) > 0
}
