package manifest

import (
	"strings"
	"testing"
)

func mustParse(t *testing.T, src string) *Manifest {
	t.Helper()
	m, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return m
}

const good = `
id: demitable.example
name: Example
version: 1.0.0
api: 1
description: An example plugin.

capabilities:
  - id: vault.read
    scope:
      types: [session]

provides:
  - id: example.panel
    kind: ui
    dom: shadow
    title: Example
    entry: ui/panel.js
    mount: sidebar.right
    when: always

consumes:
  - service: vault
    version: ">=1.0.0 <2.0.0"

config:
  - key: limit
    type: number
    default: 5
`

func TestParseAndValidateGood(t *testing.T) {
	m := mustParse(t, good)
	if errs := Validate(m); len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
	if m.ID != "demitable.example" {
		t.Fatalf("id = %q", m.ID)
	}
	if len(m.Surfaces()) != 1 || len(m.Services()) != 0 {
		t.Fatalf("surfaces/services = %d/%d", len(m.Surfaces()), len(m.Services()))
	}
}

func TestStrictProfile(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "anchors",
			src: `
id: a.b
name: x
version: 1.0.0
api: 1
config:
  - key: a
    type: text
    default: &x hello
  - key: b
    type: text
    default: *x
`,
			want: "anchors",
		},
		{
			name: "merge keys",
			src: `
id: a.b
name: x
version: 1.0.0
api: 1
<<: {api: 1}
`,
			want: "merge keys",
		},
		{
			name: "duplicate keys",
			src: `
id: a.b
name: x
name: y
version: 1.0.0
api: 1
`,
			want: "duplicate key",
		},
		{
			name: "implicit dates",
			src: `
id: a.b
name: x
version: 1.0.0
api: 1
config:
  - key: when
    type: text
    default: 2026-10-23
`,
			want: "implicit dates",
		},
		{
			name: "unknown field",
			src: `
id: a.b
name: x
version: 1.0.0
api: 1
surfaces:
  - id: s
`,
			want: "field surfaces not found",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(tc.src))
			if err == nil {
				t.Fatalf("expected an error mentioning %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

func TestValidationRules(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "unknown capability",
			src: `
id: a.b
name: x
version: 1.0.0
api: 1
capabilities:
  - id: vault.teleport
`,
			want: "not in the capability vocabulary",
		},
		{
			name: "unsupported api",
			src: `
id: a.b
name: x
version: 1.0.0
api: 2
`,
			want: "not supported by this core",
		},
		{
			name: "light surface without dom.inline",
			src: `
id: a.b
name: x
version: 1.0.0
api: 1
provides:
  - id: s
    kind: ui
    dom: light
    title: S
    entry: ui/s.js
    element: a-b-c
    mount: sidebar.left
`,
			want: "needs the dom.inline capability",
		},
		{
			name: "light surface without an element name",
			src: `
id: a.b
name: x
version: 1.0.0
api: 1
capabilities:
  - id: dom.inline
provides:
  - id: s
    kind: ui
    dom: light
    title: S
    entry: ui/s.js
    mount: sidebar.left
`,
			want: "must name the custom element",
		},
		{
			name: "unknown mount",
			src: `
id: a.b
name: x
version: 1.0.0
api: 1
provides:
  - id: s
    kind: ui
    title: S
    entry: ui/s.js
    mount: sidebar.bottom
`,
			want: "is not a mount point",
		},
		{
			name: "note.body is reserved",
			src: `
id: other.plugin
name: x
version: 1.0.0
api: 1
provides:
  - id: s
    kind: ui
    title: S
    entry: ui/s.js
    mount: note.body
`,
			want: "is reserved for the editor",
		},
		{
			name: "two surfaces on one mount",
			src: `
id: a.b
name: x
version: 1.0.0
api: 1
provides:
  - id: s1
    kind: ui
    title: S
    entry: ui/s.js
    mount: sidebar.left
  - id: s2
    kind: ui
    title: S
    entry: ui/s.js
    mount: sidebar.left
`,
			want: "already claimed",
		},
		{
			name: "a service with a surface field",
			src: `
id: a.b
name: x
version: 1.0.0
api: 1
provides:
  - id: s
    kind: service
    version: 1.0.0
    mount: sidebar.left
`,
			want: "a service provides entry has no",
		},
		{
			name: "a select with no options",
			src: `
id: a.b
name: x
version: 1.0.0
api: 1
config:
  - key: k
    type: select
    default: a
`,
			want: "a select needs options",
		},
		{
			name: "a default of the wrong type",
			src: `
id: a.b
name: x
version: 1.0.0
api: 1
config:
  - key: k
    type: number
    default: "three"
`,
			want: "must be a number",
		},
		{
			name: "a scope on a declaration capability",
			src: `
id: a.b
name: x
version: 1.0.0
api: 1
capabilities:
  - id: dom.inline
    scope:
      types: [session]
`,
			want: "cannot be scoped",
		},
		{
			name: "yes is not a boolean",
			src: `
id: a.b
name: x
version: 1.0.0
api: 1
config:
  - key: flag
    type: boolean
    default: yes
`,
			want: "must be true or false",
		},
		{
			name: "a bad plugin id",
			src: `
id: Example
name: x
version: 1.0.0
api: 1
`,
			want: "is not a valid plugin id",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, err := Parse([]byte(tc.src))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			errs := Validate(m)
			if len(errs) == 0 {
				t.Fatalf("expected an error mentioning %q", tc.want)
			}
			joined := manifestErrors(errs)
			if !strings.Contains(joined, tc.want) {
				t.Fatalf("errors = %v, want one mentioning %q", joined, tc.want)
			}
		})
	}
}

func manifestErrors(errs []error) string {
	parts := make([]string, 0, len(errs))
	for _, e := range errs {
		parts = append(parts, e.Error())
	}
	return strings.Join(parts, "; ")
}
