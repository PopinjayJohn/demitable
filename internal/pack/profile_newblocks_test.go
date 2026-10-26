package pack

import (
	"strings"
	"testing"
)

func TestProfileChecksHoldAroundNewBlocks(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "anchor or alias",
			src: `id: &a x
name: *a
version: 1.0.0
attributes:
  - { key: str, label: Strength, type: number }
content: []
`,
			want: "anchors and aliases",
		},
		{
			name: "merge key",
			src: `id: x
name: X
version: 1.0.0
resources:
  - { key: hp, label: HP, kind: pool, max: 6 }
<<: { extra: true }
content: []
`,
			want: "merge keys",
		},
		{
			name: "duplicate key",
			src: `id: x
id: y
name: X
version: 1.0.0
conditions:
  - { key: prone, label: Prone, kind: flag }
content: []
`,
			want: "duplicate key",
		},
		{
			name: "unquoted date",
			src: `id: x
name: X
version: 1.0.0
created: 2024-01-01
content: []
`,
			want: "implicit dates",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Parse([]byte(c.src))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("got %v, want %q", err, c.want)
			}
		})
	}
}
