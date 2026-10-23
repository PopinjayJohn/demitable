// Package manifest is the plugin manifest: its schema, its YAML profile, and
// its validator.
//
// The schema is defined by the Go struct in this file. The YAML tags are the
// serialisation. The compiled-in tier binds the schema as Go literals; the
// runtime tiers bind it as YAML. One schema, two bindings — and the two are
// kept honest by a round-trip test.
package manifest

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/PopinjayJohn/demitable/spikes/plugin-host/caps"
)

// Manifest is the whole of what a plugin says about itself.
//
// Every field here was put here because one of the two spike plugins needed
// it. Fields that no plugin needed are gone; see README.md §4.
type Manifest struct {
	// ID is unique across plugins, reverse-DNS-ish.
	ID string `yaml:"id"`
	// Name is the human label.
	Name string `yaml:"name"`
	// Version is the plugin's own semver, independent of API.
	Version string `yaml:"version"`
	// API is the plugin API major this plugin targets.
	//
	// ADR-0045 calls this field `abi:`. ADR-0019, ADR-0020 and
	// docs/plugin-api.md call it `api:`. This spike uses `api:` — see
	// README.md §7.
	API int `yaml:"api"`
	// Description is one line of prose.
	Description string `yaml:"description,omitempty"`
	// Author is a free-form credit.
	Author string `yaml:"author,omitempty"`
	// Homepage is a URL.
	Homepage string `yaml:"homepage,omitempty"`

	// Capabilities is what the plugin asks for.
	Capabilities []Capability `yaml:"capabilities,omitempty"`
	// Provides is what the plugin offers.
	Provides []Provides `yaml:"provides,omitempty"`
	// Consumes is what the plugin needs from other providers.
	Consumes []Consumes `yaml:"consumes,omitempty"`
	// Config is the declarative settings block the core renders.
	Config []ConfigField `yaml:"config,omitempty"`
}

// Capability is one requested capability, optionally narrowed.
type Capability struct {
	ID    string `yaml:"id"`
	Scope *Scope `yaml:"scope,omitempty"`
}

// Scope narrows a capability. The keys compose: the effective permission is
// the intersection of every declared scope.
type Scope struct {
	Paths []string `yaml:"paths,omitempty"`
	Types []string `yaml:"types,omitempty"`
	UIDs  []string `yaml:"uids,omitempty"`
}

// Empty reports whether the scope narrows nothing.
func (s *Scope) Empty() bool {
	return s == nil || (len(s.Paths) == 0 && len(s.Types) == 0 && len(s.UIDs) == 0)
}

// Provides is one thing the plugin offers.
type Provides struct {
	ID   string `yaml:"id"`
	Kind string `yaml:"kind"` // "ui" | "service"

	// --- ui only ---

	// DOM is where the surface mounts: "shadow" (default), "light" or
	// "frame".
	DOM string `yaml:"dom,omitempty"`
	// Title is the label shown in chrome and in the command palette.
	Title string `yaml:"title,omitempty"`
	// Entry is the module to load. Its meaning depends on DOM:
	//   shadow -> a JavaScript module exporting mount(host, ctx, demitable)
	//   light  -> a JavaScript module registering the custom element
	//   frame  -> an HTML document
	Entry string `yaml:"entry,omitempty"`
	// Style is a stylesheet belonging to this surface. The host puts it in
	// the shadow root for a shadow surface, and in the document (scoped) for
	// a light one.
	Style string `yaml:"style,omitempty"`
	// Element is the custom element name a light surface registers.
	Element string `yaml:"element,omitempty"`
	// Mount is the named host the surface mounts into.
	Mount string `yaml:"mount,omitempty"`
	// Icon names an icon.
	Icon string `yaml:"icon,omitempty"`
	// Order orders surfaces sharing a mount.
	Order int `yaml:"order,omitempty"`
	// When is a formula-language expression; "always" means always.
	When string `yaml:"when,omitempty"`

	// --- service only ---

	// ServiceVersion is the version of the service this plugin provides.
	ServiceVersion string `yaml:"version,omitempty"`
}

// Consumes is a dependency on another provider.
type Consumes struct {
	Service  string `yaml:"service"`
	Version  string `yaml:"version"`
	Optional bool   `yaml:"optional,omitempty"`
}

// ConfigField is one entry of the declarative settings block.
type ConfigField struct {
	Key     string   `yaml:"key"`
	Type    string   `yaml:"type"` // text | number | boolean | select | path
	Label   string   `yaml:"label,omitempty"`
	Default any      `yaml:"default,omitempty"`
	Options []string `yaml:"options,omitempty"`
}

// The mount points. Which of them are open to plugins, and which are
// reserved, is a manifest question and not a host question — see README §6.
const (
	MountNoteHeader   = "note.header"
	MountNoteSidebar  = "note.sidebar"
	MountNoteBody     = "note.body"
	MountSidebarLeft  = "sidebar.left"
	MountSidebarRight = "sidebar.right"
	MountPanelBottom  = "panel.bottom"
	MountDialog       = "dialog"
	MountCommand      = "command"
)

// MountPoints is the whole list of mount points.
var MountPoints = []string{
	MountNoteHeader,
	MountNoteSidebar,
	MountNoteBody,
	MountSidebarLeft,
	MountSidebarRight,
	MountPanelBottom,
	MountDialog,
	MountCommand,
}

// ReservedMounts are the mount points the app owns outright, mapped to the
// plugin id that may claim them.
var ReservedMounts = map[string]string{
	MountNoteBody: "demitable.editor",
}

// ReservedMountDescriptions explains each reserved mount, for the message.
var ReservedMountDescriptions = map[string]string{
	MountNoteBody: "the editor",
}

// Surface kinds.
const (
	KindService = "service"
	KindUI      = "ui"
)

// DOM kinds.
const (
	DOMShadow = "shadow"
	DOMLight  = "light"
	DOMFrame  = "frame"
)

// Config field types.
var ConfigTypes = []string{"text", "number", "boolean", "select", "path"}

// Kinds returns the recognised `kind:` values.
var Kinds = []string{KindService, KindUI}

// DOMKinds returns the recognised `dom:` values.
var DOMKinds = []string{DOMShadow, DOMLight, DOMFrame}

// idPattern is the plugin id grammar: lowercase, dotted, at least two parts.
var idPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(\.[a-z0-9][a-z0-9-]*)+$`)

// elementPattern is the custom-element name grammar.
var elementPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)+$`)

// semverPattern is a rough semver check.
var semverPattern = regexp.MustCompile(`^\d+\.\d+\.\d+`)

// SupportedAPI is the plugin API major this core speaks.
const SupportedAPI = 1

// ---------------------------------------------------------------------------
// Parsing
// ---------------------------------------------------------------------------

// Parse reads a manifest from YAML bytes under the strict profile.
func Parse(data []byte) (*Manifest, error) {
	var root yaml.Node
	dec := yaml.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&root); err != nil {
		return nil, fmt.Errorf("manifest: not valid YAML: %w", err)
	}
	if err := checkProfile(&root); err != nil {
		return nil, err
	}

	var m Manifest
	strict := yaml.NewDecoder(bytes.NewReader(data))
	strict.KnownFields(true)
	if err := strict.Decode(&m); err != nil {
		return nil, fmt.Errorf("manifest: %w", err)
	}
	return &m, nil
}

// Marshal renders a manifest as YAML, in the canonical field order.
func Marshal(m *Manifest) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(m); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ---------------------------------------------------------------------------
// The strict YAML profile
// ---------------------------------------------------------------------------

// checkProfile enforces the manifest's YAML profile: the subset of YAML that
// has one unambiguous reading. Every rule here was inherited from the
// frontmatter extractor's findings — the manifest is written by humans and
// read by machines, so it may not be ambiguous.
func checkProfile(root *yaml.Node) error {
	if root.Kind == 0 {
		return errors.New("manifest: empty document")
	}
	var walk func(n *yaml.Node, path string) error
	walk = func(n *yaml.Node, path string) error {
		switch n.Kind {
		case yaml.DocumentNode:
			for _, c := range n.Content {
				if err := walk(c, path); err != nil {
					return err
				}
			}
		case yaml.AliasNode:
			return fmt.Errorf("manifest: YAML anchors and aliases are not allowed (%s)", path)
		case yaml.MappingNode:
			seen := map[string]int{}
			for i := 0; i < len(n.Content); i += 2 {
				k, v := n.Content[i], n.Content[i+1]
				if k.Tag == "!!merge" {
					return fmt.Errorf("manifest: YAML merge keys are not allowed (%s)", path)
				}
				if k.Kind != yaml.ScalarNode {
					return fmt.Errorf("manifest: non-string key at %s", path)
				}
				if _, dup := seen[k.Value]; dup {
					return fmt.Errorf("manifest: duplicate key %q at %s", k.Value, path)
				}
				seen[k.Value] = k.Line
				if err := walk(v, path+"."+k.Value); err != nil {
					return err
				}
			}
		case yaml.SequenceNode:
			for i, c := range n.Content {
				if err := walk(c, fmt.Sprintf("%s[%d]", path, i)); err != nil {
					return err
				}
			}
		case yaml.ScalarNode:
			if n.Tag == "!!timestamp" {
				return fmt.Errorf("manifest: implicit dates are not allowed (%s = %q)", path, n.Value)
			}
			if n.Tag == "!!bool" && n.Value != "true" && n.Value != "false" {
				return fmt.Errorf("manifest: %q is not a boolean; write true or false (%s)", n.Value, path)
			}
		}
		return nil
	}
	return walk(root, "$")
}

// ---------------------------------------------------------------------------
// Validation
// ---------------------------------------------------------------------------

// Validate checks a manifest against the vocabulary, the mount points and the
// cross-field rules. It returns every problem it finds, not just the first.
func Validate(m *Manifest) []error {
	var errs []error
	add := func(format string, args ...any) {
		errs = append(errs, fmt.Errorf(format, args...))
	}

	if m.ID == "" {
		add("id: required")
	} else if !idPattern.MatchString(m.ID) {
		add("id: %q is not a valid plugin id (want lowercase, dotted, e.g. demitable.recap)", m.ID)
	}
	if m.Name == "" {
		add("name: required")
	}
	if m.Version == "" {
		add("version: required")
	} else if !semverPattern.MatchString(m.Version) {
		add("version: %q is not semver", m.Version)
	}
	if m.API == 0 {
		add("api: required")
	} else if m.API != SupportedAPI {
		add("api: %d is not supported by this core (it speaks %d)", m.API, SupportedAPI)
	}
	if m.Homepage != "" && !strings.HasPrefix(m.Homepage, "http://") && !strings.HasPrefix(m.Homepage, "https://") {
		add("homepage: %q is not an http(s) URL", m.Homepage)
	}

	// --- capabilities ---

	seenCaps := map[string]bool{}
	for i, c := range m.Capabilities {
		where := fmt.Sprintf("capabilities[%d]", i)
		if c.ID == "" {
			add("%s.id: required", where)
			continue
		}
		where = fmt.Sprintf("capabilities[%d] (%s)", i, c.ID)
		if seenCaps[c.ID] {
			add("%s: declared twice", where)
		}
		seenCaps[c.ID] = true

		v, ok := caps.ByID(c.ID)
		if !ok {
			add("%s: not in the capability vocabulary", where)
			continue
		}
		if c.Scope != nil {
			if c.Scope.Empty() {
				add("%s.scope: present but empty", where)
			}
			if v.Kind == caps.Declaration {
				add("%s.scope: a declaration capability cannot be scoped", where)
			}
		}
	}

	// --- provides ---

	seenProvides := map[string]bool{}
	seenMounts := map[string]string{}
	for i, p := range m.Provides {
		where := fmt.Sprintf("provides[%d]", i)
		if p.ID == "" {
			add("%s.id: required", where)
			continue
		}
		where = fmt.Sprintf("provides[%d] (%s)", i, p.ID)
		if seenProvides[p.ID] {
			add("%s: declared twice", where)
		}
		seenProvides[p.ID] = true

		switch p.Kind {
		case KindService:
			if p.ServiceVersion == "" {
				add("%s.version: a provided service needs a version", where)
			} else if !semverPattern.MatchString(p.ServiceVersion) {
				add("%s.version: %q is not semver", where, p.ServiceVersion)
			}
			for _, bad := range []string{p.DOM, p.Entry, p.Mount, p.Element, p.Style, p.When} {
				if bad != "" {
					add("%s: a service provides entry has no %q field", where, bad)
				}
			}
			continue
		case KindUI:
		default:
			add("%s.kind: %q is not one of %s", where, p.Kind, strings.Join(Kinds, ", "))
			continue
		}

		dom := p.DOM
		if dom == "" {
			dom = DOMShadow
		}
		switch dom {
		case DOMShadow:
			if p.Element != "" {
				add("%s.element: only a light surface registers a custom element", where)
			}
			if p.Entry == "" {
				add("%s.entry: required", where)
			} else if !strings.HasSuffix(p.Entry, ".js") {
				add("%s.entry: a shadow surface's entry is a JavaScript module, got %q", where, p.Entry)
			}
		case DOMLight:
			if p.Element == "" {
				add("%s.element: a light surface must name the custom element it registers", where)
			} else if !elementPattern.MatchString(p.Element) {
				add("%s.element: %q is not a valid custom element name", where, p.Element)
			}
			if p.Entry == "" {
				add("%s.entry: required", where)
			} else if !strings.HasSuffix(p.Entry, ".js") {
				add("%s.entry: a light surface's entry is a JavaScript module, got %q", where, p.Entry)
			}
			if !seenCaps["dom.inline"] {
				add("%s: dom: light needs the dom.inline capability", where)
			}
		case DOMFrame:
			if p.Entry == "" {
				add("%s.entry: required", where)
			} else if !strings.HasSuffix(p.Entry, ".html") {
				add("%s.entry: a frame surface's entry is an HTML document, got %q", where, p.Entry)
			}
		default:
			add("%s.dom: %q is not one of %s", where, p.DOM, strings.Join(DOMKinds, ", "))
		}

		if p.Title == "" {
			add("%s.title: required", where)
		}
		if p.Mount == "" {
			add("%s.mount: required", where)
		} else {
			if !contains(MountPoints, p.Mount) {
				add("%s.mount: %q is not a mount point", where, p.Mount)
			}
			if owner, reserved := ReservedMounts[p.Mount]; reserved && m.ID != owner {
				add("%s.mount: %q is reserved for %s", where, p.Mount, ReservedMountDescriptions[p.Mount])
			}
			if other, taken := seenMounts[p.Mount]; taken && other != p.ID {
				add("%s.mount: %q is already claimed by %q in this manifest", where, p.Mount, other)
			}
			seenMounts[p.Mount] = p.ID
		}
		if p.When != "" && p.When != "always" {
			// A real formula parser is M2's job (ADR-0003). The manifest
			// validator needs one, which is why M2 precedes M3.
			if err := checkFormulaShape(p.When); err != nil {
				add("%s.when: %v", where, err)
			}
		}
	}

	// --- consumes ---

	for i, c := range m.Consumes {
		where := fmt.Sprintf("consumes[%d]", i)
		if c.Service == "" {
			add("%s.service: required", where)
		}
		if c.Version == "" {
			add("%s.version: required", where)
		}
	}

	// --- config ---

	seenKeys := map[string]bool{}
	for i, f := range m.Config {
		where := fmt.Sprintf("config[%d]", i)
		if f.Key == "" {
			add("%s.key: required", where)
			continue
		}
		where = fmt.Sprintf("config[%d] (%s)", i, f.Key)
		if seenKeys[f.Key] {
			add("%s: declared twice", where)
		}
		seenKeys[f.Key] = true
		if !contains(ConfigTypes, f.Type) {
			add("%s.type: %q is not one of %s", where, f.Type, strings.Join(ConfigTypes, ", "))
		}
		if f.Type == "select" && len(f.Options) == 0 {
			add("%s.options: a select needs options", where)
		}
		if err := checkDefault(f); err != nil {
			add("%s.default: %v", where, err)
		}
	}

	return errs
}

func checkDefault(f ConfigField) error {
	if f.Default == nil {
		return nil
	}
	switch f.Type {
	case "text", "path":
		if _, ok := f.Default.(string); !ok {
			return fmt.Errorf("a %s field's default must be a string, got %T", f.Type, f.Default)
		}
	case "number":
		switch f.Default.(type) {
		case int, int64, float64, float32:
		default:
			return fmt.Errorf("a number field's default must be a number, got %T", f.Default)
		}
	case "boolean":
		if _, ok := f.Default.(bool); !ok {
			return fmt.Errorf("a boolean field's default must be true or false, got %T", f.Default)
		}
	case "select":
		s, ok := f.Default.(string)
		if !ok {
			return fmt.Errorf("a select field's default must be a string, got %T", f.Default)
		}
		if !contains(f.Options, s) {
			return fmt.Errorf("%q is not one of the options", s)
		}
	}
	return nil
}

// checkFormulaShape is a cheap syntactic check on a `when:` expression. The
// real parser is M2's; the spike only needs to catch an obviously broken
// expression at manifest-load time.
func checkFormulaShape(expr string) error {
	if strings.Count(expr, "(") != strings.Count(expr, ")") {
		return fmt.Errorf("unbalanced parentheses")
	}
	if strings.Count(expr, "\"")%2 != 0 {
		return fmt.Errorf("unbalanced quotes")
	}
	return nil
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Queries over a manifest
// ---------------------------------------------------------------------------

// Capabilities returns the capability ids the manifest asks for, sorted.
func (m *Manifest) CapabilityIDs() []string {
	out := make([]string, 0, len(m.Capabilities))
	for _, c := range m.Capabilities {
		out = append(out, c.ID)
	}
	sort.Strings(out)
	return out
}

// HasCapability reports whether the manifest declares a capability.
func (m *Manifest) HasCapability(id string) bool {
	for _, c := range m.Capabilities {
		if c.ID == id {
			return true
		}
	}
	return false
}

// Surfaces returns the UI surfaces the manifest provides.
func (m *Manifest) Surfaces() []Provides {
	var out []Provides
	for _, p := range m.Provides {
		if p.Kind == KindUI {
			out = append(out, p)
		}
	}
	return out
}

// Services returns the services the manifest provides.
func (m *Manifest) Services() []Provides {
	var out []Provides
	for _, p := range m.Provides {
		if p.Kind == KindService {
			out = append(out, p)
		}
	}
	return out
}

// DOMKind returns a surface's dom kind, defaulting to shadow.
func (p Provides) DOMKind() string {
	if p.DOM == "" {
		return DOMShadow
	}
	return p.DOM
}

// Errors is a list of validation errors.
type Errors []error

func (e Errors) Error() string {
	parts := make([]string, 0, len(e))
	for _, err := range e {
		parts = append(parts, err.Error())
	}
	return strings.Join(parts, "; ")
}
