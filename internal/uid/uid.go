// Package uid generates Demitable's note identifiers.
//
// A uid is a 26-character Crockford base32 string: 10 characters encode a
// 48-bit millisecond timestamp (so uids sort by creation time) and 16
// characters encode 80 bits of cryptographic randomness. It is not derived
// from the note's name or any other content.
//
// Generation uses crypto/rand and has no fallback: if the platform cannot
// provide randomness, generation fails rather than returning a predictable
// value.
package uid

import (
	"crypto/rand"
	"fmt"
	"time"
)

const alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// New returns a fresh uid. It fails if crypto/rand cannot provide
// randomness.
func New() (string, error) {
	return NewAt(time.Now())
}

// NewAt returns a uid stamped with the given time. It is exposed for
// deterministic callers (such as the SRD pipeline); production code uses
// New.
func NewAt(t time.Time) (string, error) {
	ts := uint64(t.UnixMilli())
	var r [10]byte
	if _, err := rand.Read(r[:]); err != nil {
		return "", fmt.Errorf("uid: crypto/rand: %w", err)
	}
	out := make([]byte, 26)
	encodeTime(ts, out[:10])
	encodeRandom(r[:], out[10:])
	return string(out), nil
}

func encodeTime(ts uint64, dst []byte) {
	for i := 9; i >= 0; i-- {
		dst[i] = alphabet[ts&0x1F]
		ts >>= 5
	}
}

func encodeRandom(src []byte, dst []byte) {
	bitPos := 0
	for i := 0; i < len(dst); i++ {
		idx := 0
		for j := 0; j < 5; j++ {
			byteIndex := bitPos / 8
			bitIndex := 7 - (bitPos % 8)
			if byteIndex < len(src) && (src[byteIndex]>>(bitIndex))&1 == 1 {
				idx |= 1 << (4 - j)
			}
			bitPos++
		}
		dst[i] = alphabet[idx]
	}
}
