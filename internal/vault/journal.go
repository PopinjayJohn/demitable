package vault

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Journal is an append-only, content-addressed write journal inside the vault.
type Journal struct {
	Root string // vault root
}

// Change describes one field change in a journal entry.
type Change struct {
	Key  string      `json:"key"`
	From interface{} `json:"from,omitempty"`
	To   interface{} `json:"to,omitempty"`
}

// Entry is one journal record.
type Entry struct {
	ID      string    `json:"id"`
	TS      time.Time `json:"ts"`
	Actor   string    `json:"actor"`
	Source  string    `json:"source"`
	Op      string    `json:"op"`
	UID     string    `json:"uid"`
	Path    string    `json:"path"`
	BatchID string    `json:"batchId,omitempty"`
	Before  string    `json:"before"` // sha256:...
	After   string    `json:"after"`
	Changes []Change  `json:"changes,omitempty"`
}

func NewJournal(root string) *Journal { return &Journal{Root: root} }

func (j *Journal) dir() string     { return filepath.Join(j.Root, ".demitable", "journal") }
func (j *Journal) blobDir() string { return filepath.Join(j.Root, ".demitable", "blobs") }

func (j *Journal) ensureDirs() error {
	if err := os.MkdirAll(j.dir(), 0o755); err != nil {
		return err
	}
	return os.MkdirAll(j.blobDir(), 0o755)
}

// Append writes a journal entry. The before/after content blobs are stored
// content-addressed if not already present.
func (j *Journal) Append(e Entry) error {
	if err := j.ensureDirs(); err != nil {
		return err
	}
	if e.TS.IsZero() {
		e.TS = time.Now().UTC()
	}
	if e.ID == "" {
		e.ID = newEntryID()
	}
	// store blobs: caller provides Before/After as "sha256:..."? We expect
	// e.BeforeContent/e.AfterContent? To keep simple, Entry only has hashes.
	// We'll add content storage via helper methods instead.
	day := e.TS.UTC().Format("2006-01-02")
	f, err := os.OpenFile(filepath.Join(j.dir(), day+".log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	return enc.Encode(e)
}

// AppendWrite records an app-initiated write: stores before/after blobs.
func (j *Journal) AppendWrite(actor, source, op, uid, relPath, batchID string, beforeContent, afterContent []byte, changes []Change) (Entry, error) {
	if err := j.ensureDirs(); err != nil {
		return Entry{}, err
	}
	beforeHash, err := j.storeBlob(beforeContent)
	if err != nil {
		return Entry{}, err
	}
	afterHash, err := j.storeBlob(afterContent)
	if err != nil {
		return Entry{}, err
	}
	e := Entry{
		ID:      newEntryID(),
		TS:      time.Now().UTC(),
		Actor:   actor,
		Source:  source,
		Op:      op,
		UID:     uid,
		Path:    relPath,
		BatchID: batchID,
		Before:  "sha256:" + beforeHash,
		After:   "sha256:" + afterHash,
		Changes: changes,
	}
	if err := j.Append(e); err != nil {
		return Entry{}, err
	}
	// re-read to get ID? Append sets ID before encode but Entry passed by value not updated. Fix: set ID before.
	return e, nil
}

func (j *Journal) storeBlob(content []byte) (string, error) {
	sum := sha256.Sum256(content)
	hash := hex.EncodeToString(sum[:])
	p := filepath.Join(j.blobDir(), hash)
	if _, err := os.Stat(p); err == nil {
		return hash, nil
	}
	return hash, os.WriteFile(p, content, 0o644)
}

// List returns all entries across days, sorted by ts.
func (j *Journal) List() ([]Entry, error) {
	if err := j.ensureDirs(); err != nil {
		return nil, err
	}
	var entries []Entry
	matches, _ := filepath.Glob(filepath.Join(j.dir(), "*.log"))
	for _, m := range matches {
		f, err := os.Open(m)
		if err != nil {
			continue
		}
		dec := json.NewDecoder(f)
		for dec.More() {
			var e Entry
			if err := dec.Decode(&e); err != nil {
				break
			}
			entries = append(entries, e)
		}
		f.Close()
	}
	sort.Slice(entries, func(i, k int) bool { return entries[i].TS.Before(entries[k].TS) })
	return entries, nil
}

// Revert restores the prior bytes for one entry.
func (j *Journal) Revert(id string) error {
	entries, err := j.List()
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.ID != id {
			continue
		}
		// read before blob
		hash := strings.TrimPrefix(e.Before, "sha256:")
		data, err := os.ReadFile(filepath.Join(j.blobDir(), hash))
		if err != nil {
			return err
		}
		full := filepath.Join(j.Root, filepath.FromSlash(e.Path))
		if err := os.WriteFile(full, data, 0o644); err != nil {
			return err
		}
		// append revert entry
		_, _ = j.AppendWrite(e.Actor, "revert", "revert", e.UID, e.Path, "", data, data, nil)
		return nil
	}
	return fmt.Errorf("journal: entry not found: %s", id)
}

// RevertBatch reverts every entry sharing batchId, in reverse order.
func (j *Journal) RevertBatch(batchID string) error {
	entries, err := j.List()
	if err != nil {
		return err
	}
	var batch []Entry
	for _, e := range entries {
		if e.BatchID == batchID {
			batch = append(batch, e)
		}
	}
	// reverse order
	for i := len(batch) - 1; i >= 0; i-- {
		e := batch[i]
		hash := strings.TrimPrefix(e.Before, "sha256:")
		data, err := os.ReadFile(filepath.Join(j.blobDir(), hash))
		if err != nil {
			return err
		}
		full := filepath.Join(j.Root, filepath.FromSlash(e.Path))
		if err := os.WriteFile(full, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func init() { /* noop */ }

var entryCounter uint64

func newEntryID() string {
	entryCounter++
	return fmt.Sprintf("je%d", entryCounter)
}
