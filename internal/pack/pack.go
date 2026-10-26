// Package pack loads and validates the declarative YAML packs that describe a
// game system's mechanics and content shapes.
//
// A pack is data, not code (ADR-0003, ADR-0023): it declares content types
// and the typed fields they carry, plus the visibility default each content
// type contributes to the projection. Phase 2 completes the format with
// resources, attributes, conditions, progression, difficulty, views,
// formulas, secrets and dice; every block is additive, so a phase-1 pack
// (no phase-2 blocks) keeps loading unchanged. The package also hosts the
// pack linter (ADR-0007, ADR-0042, ADR-0044) that checks a pack's
// declarations and formula references at load time.
package pack

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/PopinjayJohn/demitable/internal/formula"

	"gopkg.in/yaml.v3"
)

// Pack is a loaded system pack.
type Pack struct {
	ID      string        `yaml:"id"`
	Name    string        `yaml:"name"`
	Version string        `yaml:"version"`
	Content []ContentType `yaml:"content"`

	Formula     any               `yaml:"formula"`
	Formulas    map[string]string `yaml:"formulas"`
	Dice        *DiceConfig       `yaml:"dice"`
	Resources   []Resource        `yaml:"resources"`
	Attributes  []Attribute       `yaml:"attributes"`
	Conditions  []Condition       `yaml:"conditions"`
	Progression *Progression      `yaml:"progression"`
	Difficulty  map[string]any    `yaml:"difficulty"`
	Views       map[string]any    `yaml:"views"`
	Secrets     *SecretsConfig    `yaml:"secrets"`
}

type Issue = formula.Issue

type ContentType struct {
	ID         string  `yaml:"id"`
	Label      string  `yaml:"label"`
	Visibility string  `yaml:"visibility"`
	Fields     []Field `yaml:"fields"`
	Choices    any     `yaml:"choices"`
	Index      any     `yaml:"index"`
}

type Field struct {
	Key        string `yaml:"key"`
	Label      string `yaml:"label"`
	Type       string `yaml:"type"`
	Required   bool   `yaml:"required"`
	Of         string `yaml:"of"`
	Visibility string `yaml:"visibility"`
	Default    any    `yaml:"default"`
}

type Attribute struct {
	Key     string `yaml:"key"`
	Label   string `yaml:"label"`
	Type    string `yaml:"type"`
	Default any    `yaml:"default"`
	Min     any    `yaml:"min"`
	Max     any    `yaml:"max"`
}

type Resource struct {
	Key        string `yaml:"key"`
	Label      string `yaml:"label"`
	Kind       string `yaml:"kind"`
	Default    any    `yaml:"default"`
	MaxFormula string `yaml:"maxFormula"`
	Max        any    `yaml:"max"`
	Min        any    `yaml:"min"`
	Value      any    `yaml:"value"`
	Size       int    `yaml:"size"`
	Segments   int    `yaml:"segments"`
	Filled     int    `yaml:"filled"`
	Visibility string `yaml:"visibility"`
}

type Condition struct {
	Key   string `yaml:"key"`
	Label string `yaml:"label"`
	Kind  string `yaml:"kind"`
	Steps []any  `yaml:"steps"`
	Value any    `yaml:"value"`
}

type Progression struct {
	Levels *Levels `yaml:"levels"`
	Ranks  any     `yaml:"ranks"`
	XP     any     `yaml:"xp"`
}

type Levels struct {
	Max   int    `yaml:"max"`
	Label string `yaml:"label"`
}

type SecretsConfig struct {
	CalloutTypes  []string `yaml:"calloutTypes"`
	CodeBlockKeys []string `yaml:"codeBlockKeys"`
}

var AllowedFieldTypes = []string{
	"text", "string", "number", "int", "integer", "float",
	"boolean", "bool", "list", "object", "ref", "link",
	"wikilink", "date", "enum", "ability", "skill",
	"condition", "resource", "choice", "map",
}

var AllowedVisibility = []string{
	"public", "party", "owner", "private", "discovered", "gm",
}

func Parse(data []byte) (*Pack, error) {
	var root yaml.Node
	dec := yaml.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&root); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("pack: empty document")
		}
		return nil, fmt.Errorf("pack: not valid YAML: %w", err)
	}
	if err := checkProfile(&root); err != nil {
		return nil, err
	}

	var p Pack
	strict := yaml.NewDecoder(bytes.NewReader(data))
	strict.KnownFields(true)
	if err := strict.Decode(&p); err != nil {
		return nil, fmt.Errorf("pack: %w", err)
	}

	if err := checkPhase2(&p); err != nil {
		return nil, err
	}
	if err := validatePack(&p); err != nil {
		return nil, err
	}
	for _, is := range p.lintFormulas() {
		if is.Severity == "error" {
			return nil, fmt.Errorf("pack: %s", is.Message)
		}
	}
	for _, is := range p.lintInvariants() {
		if is.Severity == "error" {
			return nil, fmt.Errorf("pack: %s", is.Message)
		}
	}
	for _, is := range p.lintDice() {
		if is.Severity == "error" {
			return nil, fmt.Errorf("pack: %s", is.Message)
		}
	}
	if errs := p.validateBlocks(); len(errs) > 0 {
		return nil, errs[0]
	}
	return &p, nil
}

func checkPhase2(p *Pack) error {
	if p.Formula != nil {
		return fmt.Errorf("pack: %q is a phase 2 feature and is not available in phase 1", "formula")
	}
	return nil
}

func validatePack(p *Pack) error {
	var errs []string
	add := func(format string, args ...any) {
		errs = append(errs, fmt.Sprintf(format, args...))
	}

	if p.ID == "" {
		add("id: required")
	}
	if p.Name == "" {
		add("name: required")
	}
	if p.Version == "" {
		add("version: required")
	}

	seenCT := map[string]bool{}
	for ci, ct := range p.Content {
		where := fmt.Sprintf("content[%d]", ci)
		if ct.ID == "" {
			add("%s.id: required", where)
		} else {
			if seenCT[ct.ID] {
				add("content type %q declared twice", ct.ID)
			}
			seenCT[ct.ID] = true
		}
		if ct.Label == "" {
			add("%s (%s): label is required", where, ct.ID)
		}
		if ct.Visibility != "" && !containsStr(AllowedVisibility, ct.Visibility) {
			add("%s (%s): visibility %q is not in the vocabulary", where, ct.ID, ct.Visibility)
		}
		seenField := map[string]bool{}
		for fi, f := range ct.Fields {
			fwhere := fmt.Sprintf("%s.fields[%d]", where, fi)
			if f.Key == "" {
				add("%s: key is required", fwhere)
				continue
			}
			if f.Type == "" {
				add("%s (%s): type is required", fwhere, f.Key)
				continue
			}
			if !containsStr(AllowedFieldTypes, f.Type) {
				add("%s (%s): unknown field type %q", fwhere, f.Key, f.Type)
			}
			if seenField[f.Key] {
				add("%s (%s): declared twice", fwhere, f.Key)
			}
			seenField[f.Key] = true
			if f.Visibility != "" && !containsStr(AllowedVisibility, f.Visibility) {
				add("%s (%s): visibility %q is not in the vocabulary", fwhere, f.Key, f.Visibility)
			}
		}
	}

	seenAttr := map[string]bool{}
	for i, a := range p.Attributes {
		where := fmt.Sprintf("attributes[%d]", i)
		if a.Key == "" {
			add("%s: key is required", where)
			continue
		}
		if seenAttr[a.Key] {
			add("attribute %q declared twice", a.Key)
		}
		seenAttr[a.Key] = true
	}
	seenRes := map[string]bool{}
	for i, r := range p.Resources {
		where := fmt.Sprintf("resources[%d]", i)
		if r.Key == "" {
			add("%s: key is required", where)
			continue
		}
		if seenRes[r.Key] {
			add("resource %q declared twice", r.Key)
		}
		seenRes[r.Key] = true
		if r.Visibility != "" && !containsStr(AllowedVisibility, r.Visibility) {
			add("%s (%s): visibility %q is not in the vocabulary", where, r.Key, r.Visibility)
		}
	}
	seenCond := map[string]bool{}
	for i, c := range p.Conditions {
		where := fmt.Sprintf("conditions[%d]", i)
		if c.Key == "" {
			add("%s: key is required", where)
			continue
		}
		if seenCond[c.Key] {
			add("condition %q declared twice", c.Key)
		}
		seenCond[c.Key] = true
	}
	if p.Secrets != nil {
		for _, ct := range p.Secrets.CalloutTypes {
			if strings.TrimSpace(ct) == "" {
				add("secrets.calloutTypes: empty entry")
			}
		}
		for _, k := range p.Secrets.CodeBlockKeys {
			if strings.TrimSpace(k) == "" {
				add("secrets.codeBlockKeys: empty entry")
			}
		}
	}

	if len(errs) > 0 {
		return errors.New("pack: " + strings.Join(errs, "; "))
	}
	return nil
}

func (p *Pack) validateBlocks() []error {
	var errs []error
	add := func(format string, args ...any) {
		errs = append(errs, fmt.Errorf("pack: "+format, args...))
	}
	for _, r := range p.Resources {
		if r.Kind == "track" && r.Size <= 0 {
			add("resource %q is a track but has no size", r.Key)
		}
	}
	for _, c := range p.Conditions {
		if c.Kind == "scale" && len(c.Steps) == 0 {
			add("condition %q is a scale but has no steps", c.Key)
		}
	}
	if p.Progression != nil && p.Progression.Levels != nil {
		if p.Progression.Levels.Max < 0 {
			add("progression.levels.max must be non-negative")
		}
	}
	return errs
}

func (p *Pack) lintFormulas() []Issue {
	if len(p.Formulas) == 0 {
		// Still run with an empty formula map? No: nothing to check.
		return nil
	}
	entity := []string{}
	if p.Attributes != nil {
		entity = append(entity, "attributes")
		for _, a := range p.Attributes {
			if a.Key != "" {
				entity = append(entity, a.Key)
			}
		}
	}
	if p.Resources != nil {
		entity = append(entity, "resources")
		for _, r := range p.Resources {
			if r.Key != "" {
				entity = append(entity, r.Key)
			}
		}
	}
	if p.Conditions != nil {
		entity = append(entity, "conditions")
		for _, c := range p.Conditions {
			if c.Key != "" {
				entity = append(entity, c.Key)
			}
		}
	}
	if p.Progression != nil {
		entity = append(entity, "level")
	}
	pack := []string{"proficiency", "spell_dc_base", "hit_die"}
	if p.Progression != nil {
		pack = append(pack, "progression")
	}
	if p.Difficulty != nil {
		pack = append(pack, "difficulty")
	}
	// Derived formula names are a layer that can shadow entity-scope names;
	// pass them as pack-scope names so formula.Check's shadowing branch can
	// fire on real declarations (ADR-0042).
	for name := range p.Formulas {
		if name != "" {
			pack = append(pack, name)
		}
	}
	call := []string{"weapon", "target", "actor", "round", "dc"}
	ctx := formula.Context{Name: "default", Entity: entity, Pack: pack, Call: call}
	fm := make(map[string]string, len(p.Formulas))
	for k, v := range p.Formulas {
		fm[k] = v
	}
	return formula.Check(fm, ctx)
}

func toPack(x any) (*Pack, error) {
	switch v := x.(type) {
	case *Pack:
		return v, nil
	case Pack:
		cp := v
		return &cp, nil
	case []byte:
		return Parse(v)
	case string:
		return Parse([]byte(v))
	}
	return nil, fmt.Errorf("pack: unsupported lint input %T", x)
}

func Lint(x any) []Issue {
	p, err := toPack(x)
	if err != nil {
		return []Issue{{Severity: "error", Kind: "invalid-input", Message: err.Error()}}
	}
	if p == nil {
		return []Issue{{Severity: "error", Kind: "invalid-input", Message: "pack: nil pack"}}
	}
	var issues []Issue
	if err := validatePack(p); err != nil {
		issues = append(issues, Issue{Severity: "error", Kind: "validation", Message: err.Error()})
	}
	for _, e := range p.validateBlocks() {
		issues = append(issues, Issue{Severity: "error", Kind: "validation", Message: e.Error()})
	}
	issues = append(issues, p.lintFormulas()...)
	issues = append(issues, p.lintInvariants()...)
	issues = append(issues, p.lintDice()...)
	return issues
}

func LintPack(x any) []error   { return issuesToErrors(Lint(x)) }
func LintErrors(x any) []error { return issuesToErrors(Lint(x)) }
func IssuesToErrors(issues []Issue) []error {
	var errs []error
	for _, is := range issues {
		errs = append(errs, fmt.Errorf("%s", is.Message))
	}
	return errs
}
func issuesToErrors(issues []Issue) []error { return IssuesToErrors(issues) }

func (p *Pack) Lint() []Issue { return Lint(p) }

// lintInvariants enforces ADR-0007's schema invariants.
func (p *Pack) lintInvariants() []Issue {
	var issues []Issue
	// Invariant 6: every pack must be able to express a player character, an
	// NPC and a creature. A pack with no content types cannot express any of
	// them.
	if len(p.Content) == 0 {
		issues = append(issues, Issue{
			Severity: "error",
			Kind:     "invariant",
			Message:  "pack: invariant 6 (every pack must express a player character, an NPC, and a creature): content: at least one content type is required",
		})
	}
	// Invariant 7: difficulty is modelled by the pack in its own terms.
	if p.Difficulty != nil {
		model, ok := p.Difficulty["model"].(string)
		if !ok || strings.TrimSpace(model) == "" {
			issues = append(issues, Issue{
				Severity: "error",
				Kind:     "invariant",
				Message:  "pack: invariant 7 (difficulty is modelled by the pack in its own terms): difficulty.model is required",
			})
		}
	}
	// Wrong-shape hint (ADR-0044): a clock encoded as a pool with max: 6.
	for _, r := range p.Resources {
		if r.Kind == "pool" {
			isSix := false
			switch v := r.Max.(type) {
			case int:
				isSix = v == 6
			case float64:
				isSix = v == 6
			case string:
				isSix = strings.TrimSpace(v) == "6"
			}
			if isSix && strings.TrimSpace(r.MaxFormula) == "" {
				issues = append(issues, Issue{
					Severity: "warning",
					Kind:     "shape",
					Formula:  r.Key,
					Message:  fmt.Sprintf("pack: resource %q looks like a clock encoded as a pool with max: 6; consider kind: track with size", r.Key),
				})
			}
		}
	}
	return issues
}

// lintDice checks dice workflows against declared resources.
func (p *Pack) lintDice() []Issue {
	var issues []Issue
	if p.Dice == nil {
		return nil
	}
	hasResources := p.Resources != nil
	resKeys := map[string]bool{}
	for _, r := range p.Resources {
		resKeys[r.Key] = true
	}
	workflowKeys := map[string]bool{}
	if p.Dice != nil {
		for _, w := range p.Dice.Workflows {
			if w.Key != "" {
				workflowKeys[w.Key] = true
			}
		}
	}
	isSimpleIdent := func(s string) bool {
		if s == "" {
			return false
		}
		for i, r := range s {
			if i == 0 {
				if !(r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
					return false
				}
			} else {
				if !(r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
					return false
				}
			}
		}
		return true
	}
	checkRef := func(field, ref string) {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			return
		}
		if strings.HasPrefix(ref, "resources.") {
			if !hasResources {
				issues = append(issues, Issue{
					Severity: "error",
					Kind:     "invariant",
					Message:  fmt.Sprintf("pack: invariant 1 (a pack need not define resources, but this pack assumes one exists): %s %q references a resource but the pack declares none", field, ref),
				})
				return
			}
			key := strings.TrimPrefix(ref, "resources.")
			if key != "" && !resKeys[key] {
				issues = append(issues, Issue{
					Severity: "error",
					Kind:     "unresolvable",
					Message:  fmt.Sprintf("pack: %s %q references undeclared resource %q", field, ref, key),
				})
			}
			return
		}
		if !isSimpleIdent(ref) {
			return
		}
		if workflowKeys[ref] {
			return
		}
		if _, ok := p.Formulas[ref]; ok {
			return
		}
		switch ref {
		case "proficiency", "level",
			"attack_bonus", "save_bonus", "check_bonus",
			"dc", "spell_dc", "to_hit",
			"damage_avg", "damage_mod",
			"damage", "success":
			return
		}
		issues = append(issues, Issue{
			Severity: "error",
			Kind:     "unresolvable",
			Message:  fmt.Sprintf("pack: %s %q references undeclared formula %q", field, ref, ref),
		})
	}
	for i, w := range p.Dice.Workflows {
		where := fmt.Sprintf("dice.workflows[%d]", i)
		checkRef(where+".against", w.Against)
		checkRef(where+".bonus", w.Bonus)
		checkRef(where+".vs", w.Vs)
		checkRef(where+".onSuccess", w.OnSuccess)
	}
	return issues
}

func Validate(x any) error {
	p, err := toPack(x)
	if err != nil {
		return err
	}
	if p == nil {
		return errors.New("pack: nil pack")
	}
	if err := checkPhase2(p); err != nil {
		return err
	}
	if err := validatePack(p); err != nil {
		return err
	}
	for _, is := range p.lintFormulas() {
		if is.Severity == "error" {
			return fmt.Errorf("pack: %s", is.Message)
		}
	}
	if errs := p.validateBlocks(); len(errs) > 0 {
		return errs[0]
	}
	return nil
}

func (ct ContentType) VisibilityDefault() string {
	if ct.Visibility == "" {
		return "gm"
	}
	return ct.Visibility
}

func VisibilityDefault(ct ContentType) string {
	if ct.Visibility == "" {
		return "gm"
	}
	return ct.Visibility
}

func (ct ContentType) ValidateNote(fm map[string]any) error {
	return validateNote(ct, fm)
}

func ValidateNote(ct ContentType, fm map[string]any) error { return validateNote(ct, fm) }

func validateNote(ct ContentType, fm map[string]any) error {
	if fm == nil {
		fm = map[string]any{}
	}
	byKey := map[string]Field{}
	for _, f := range ct.Fields {
		byKey[f.Key] = f
	}
	var errs []string
	add := func(format string, args ...any) {
		errs = append(errs, fmt.Sprintf(format, args...))
	}

	for _, f := range ct.Fields {
		if f.Required {
			if _, ok := fm[f.Key]; !ok {
				add("field %q is required", f.Key)
			}
		}
	}

	for key, val := range fm {
		f, declared := byKey[key]
		if !declared {
			continue
		}
		if val == nil {
			continue
		}
		if !typeMatches(f.Type, val) {
			add("field %q has wrong type: want %s", key, f.Type)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("pack: %s", strings.Join(errs, "; "))
	}
	return nil
}

func typeMatches(ftype string, v any) bool {
	switch ftype {
	case "text", "string", "ref", "link", "wikilink", "date", "enum",
		"ability", "skill", "condition", "resource", "choice":
		_, ok := v.(string)
		return ok
	case "number", "int", "integer", "float":
		switch v.(type) {
		case int, int8, int16, int32, int64,
			uint, uint8, uint16, uint32, uint64,
			float32, float64:
			return true
		}
		return false
	case "boolean", "bool":
		_, ok := v.(bool)
		return ok
	case "list":
		switch v.(type) {
		case []any, []string, []int, []float64:
			return true
		}
		return isSlice(v)
	case "object", "map":
		_, ok := v.(map[string]any)
		if ok {
			return true
		}
		_, ok = v.(map[any]any)
		return ok
	}
	return true
}

func isSlice(v any) bool {
	switch v.(type) {
	case []any:
		return true
	}
	return false
}

func Marshal(p *Pack) ([]byte, error) {
	if p == nil {
		return nil, errors.New("pack: nil pack")
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(p); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func containsStr(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
