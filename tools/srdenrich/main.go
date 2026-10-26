// Command srdenrich enriches the SRD markdown corpus into a content vault.
//
// Usage:
//
//	srdenrich -fixtures fixtures -out dev/srd-vault
//	srdenrich -fixtures fixtures -out dev/srd-vault -check
//
// The corpus is a transient build input (ADR-0024). The pipeline is
// deterministic: same input produces byte-identical output.
package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	fixturesFlag = flag.String("fixtures", "fixtures", "corpus directory")
	outFlag      = flag.String("out", "dev/srd-vault", "output vault directory")
	checkFlag    = flag.Bool("check", false, "verify output matches committed vault")
	reportFlag   = flag.String("report", "", "write build report JSON here")
	archiveFlag  = flag.String("archive", "", "write importable archive zip here")
)

type entity struct {
	Type      string
	Name      string
	Source    string
	Data      map[string]any
	Converted int
}

func main() {
	flag.Parse()
	root, _ := filepath.Abs(*fixturesFlag)
	if r, err := filepath.EvalSymlinks(root); err == nil {
		root = r
	}
	out, _ := filepath.Abs(*outFlag)

	// Collect markdown files.
	var mdFiles []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(p, ".md") {
			return nil
		}
		if strings.Contains(p, string(filepath.Separator)+"data"+string(filepath.Separator)+"generated"+string(filepath.Separator)) {
			return nil
		}
		mdFiles = append(mdFiles, p)
		return nil
	})
	if err != nil {
		fail(err)
	}
	sort.Strings(mdFiles)
	// Load JSON entities.
	reg := loadEntities(root)

	// Build output in temp dir for check mode.
	tmpOut := out
	if *checkFlag {
		td, err := os.MkdirTemp("", "srdenrich-*")
		if err != nil {
			fail(err)
		}
		defer os.RemoveAll(td)
		tmpOut = td
	}

	if err := os.MkdirAll(tmpOut, 0o755); err != nil {
		fail(err)
	}

	// Clean output dir (only md files? remove whole dir contents for determinism)
	// We'll remove existing md to avoid stale.
	filepath.WalkDir(tmpOut, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			os.Remove(p)
		}
		return nil
	})

	var (
		emitted   int
		joined    int
		unjoined  []string
		converted int
	)
	// Track unused JSON entities.
	usedKeys := map[string]bool{}

	for _, p := range mdFiles {
		rel, _ := filepath.Rel(root, p)
		// read
		raw, err := os.ReadFile(p)
		if err != nil {
			fail(err)
		}
		// parse frontmatter via simple split
		fm, body := splitFrontmatter(raw)
		// Drop links to non-SRD (phb-sourced) list pages that are not emitted.
		body = unwrapDanglingLinks(body)
		src := sourceFrom(fm, p)
		if src == "" {
			src = "xphb"
		}
		// skip non-SRD sources
		if src != "xphb" && src != "xdmg" && src != "xmm" {
			continue
		}
		name := nameFrom(fm, p)
		ct := contentTypeFrom(p, fm)
		// join
		key := srdTypeFor(ct) + "|" + strings.ToLower(name) + "|" + src
		ent, ok := reg[key]
		if ok {
			usedKeys[key] = true
			joined++
		} else {
			unjoined = append(unjoined, rel)
		}
		uid := deriveUID(rel)
		// build data
		data := map[string]any{}
		if ok {
			data = cloneMap(ent.Data)
		}
		if ok {
			converted += ent.Converted
		}
		// also top-level mechanics (duplicate for compatibility)
		// Build new frontmatter map preserving original fields.
		newFM := map[string]any{}
		for k, v := range fm {
			newFM[k] = v
		}
		newFM["type"] = "content"
		newFM["content_type"] = ct
		newFM["uid"] = uid
		newFM["name"] = name
		newFM["system"] = "dnd5e"
		newFM["visibility"] = "party"
		newFM["src"] = src
		newFM["source"] = "SRD 5.2.1"
		newFM["license"] = "CC-BY-4.0"
		newFM["attribution"] = "Wizards of the Coast — D&D System Reference Document 5.2.1"
		newFM["data"] = data
		// write
		outPath := filepath.Join(tmpOut, rel)
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			fail(err)
		}
		var buf bytes.Buffer
		buf.WriteString("---\n")
		enc := yaml.NewEncoder(&buf)
		enc.SetIndent(2)
		if err := enc.Encode(newFM); err != nil {
			fail(err)
		}
		enc.Close()
		buf.WriteString("---\n")
		buf.Write(body)
		if err := os.WriteFile(outPath, buf.Bytes(), 0o644); err != nil {
			fail(err)
		}
		emitted++
	}

	// Unused entities: JSON entities with x-sourced source not used.
	var unused []string
	for key := range reg {
		parts := strings.SplitN(key, "|", 3)
		if len(parts) < 3 {
			continue
		}
		src := parts[2]
		if src != "xphb" && src != "xdmg" && src != "xmm" {
			continue
		}
		if !usedKeys[key] {
			unused = append(unused, key)
		}
	}
	sort.Strings(unused)
	sort.Strings(unjoined)

	report := map[string]any{
		"emitted":   emitted,
		"joined":    joined,
		"unjoined":  len(unjoined),
		"unused":    len(unused),
		"converted": converted,
	}
	reportJSON, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(reportJSON))
	if *reportFlag != "" {
		os.WriteFile(*reportFlag, reportJSON, 0o644)
	}

	// Write unjoined/unused lists maybe to stderr
	fmt.Fprintf(os.Stderr, "unjoined: %d, unused: %d, converted: %d\n", len(unjoined), len(unused), converted)

	if *checkFlag {
		// compare tmpOut to out
		if err := compareDirs(tmpOut, out); err != nil {
			fmt.Fprintln(os.Stderr, "CHECK FAILED:", err)
			os.Exit(1)
		}
		fmt.Println("CHECK OK: byte-identical")
	}

	if *archiveFlag != "" {
		if err := writeArchive(tmpOut, *archiveFlag); err != nil {
			fail(err)
		}
		fmt.Println("archive written:", *archiveFlag)
	}
}

var (
	reDanglingEmbed  = regexp.MustCompile(`!\[([^\]]*)\]\([^)]*list-optfeaturetype-(?:ei|mm)\.md[^)]*\)`)
	reDanglingLink   = regexp.MustCompile(`\[([^\]]*)\]\([^)]*list-optfeaturetype-(?:ei|mm)\.md[^)]*\)`)
	reDanglingAnchor = regexp.MustCompile(`<a href="[^"]*list-optfeaturetype-(?:ei|mm)\.md[^"]*">([^<]*)</a>`)
)

func unwrapDanglingLinks(body []byte) []byte {
	out := reDanglingAnchor.ReplaceAll(body, []byte("$1"))
	out = reDanglingEmbed.ReplaceAll(out, []byte("$1"))
	out = reDanglingLink.ReplaceAll(out, []byte("$1"))
	return out
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "srdenrich:", err)
	os.Exit(1)
}

func splitFrontmatter(raw []byte) (map[string]any, []byte) {
	// find --- delimiters
	text := string(raw)
	if !strings.HasPrefix(text, "---") {
		return map[string]any{}, raw
	}
	// find closing
	lines := strings.Split(text, "\n")
	// first line delimiter
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return map[string]any{}, raw
	}
	inner := strings.Join(lines[1:end], "\n")
	var fm map[string]any
	if err := yaml.Unmarshal([]byte(inner), &fm); err != nil || fm == nil {
		fm = map[string]any{}
	}
	// body: after closing delimiter line
	bodyLines := lines[end+1:]
	body := strings.Join(bodyLines, "\n")
	// ensure leading newline? original body includes after ---\n
	// Reconstruct body bytes exactly: everything after closing --- line including newline.
	// find closing delimiter
	closing := "\n---\n"
	pos := strings.Index(text, closing)
	if pos >= 0 {
		body = text[pos+len(closing):]
	} else {
		// maybe no trailing newline
		closing2 := "\n---"
		pos2 := strings.Index(text, closing2)
		if pos2 >= 0 {
			body = text[pos2+len(closing2):]
			if strings.HasPrefix(body, "\n") {
				body = body[1:]
			}
		}
	}
	return fm, []byte(body)
}

func asStringSlice(v any) []string {
	var out []string
	switch t := v.(type) {
	case []any:
		for _, e := range t {
			switch v := e.(type) {
			case string:
				out = append(out, v)
			case float64:
				out = append(out, str(v))
			case map[string]any:
				for _, key := range []string{"name", "id", "label", "title"} {
					if s, ok := v[key].(string); ok && s != "" {
						out = append(out, s)
						break
					}
				}
			default:
				// Skip non-scalar elements rather than leaking Go syntax.
			}
		}
	case []string:
		out = t
	}
	return out
}

func sourceFrom(fm map[string]any, p string) string {
	for _, t := range asStringSlice(fm["tags"]) {
		if strings.HasPrefix(t, "compendium/src/5e/") {
			return strings.TrimPrefix(t, "compendium/src/5e/")
		}
	}
	// filename suffix
	base := filepath.Base(p)
	name := strings.TrimSuffix(base, ".md")
	for _, src := range []string{"xphb", "xdmg", "xmm", "phb", "dmg", "mm"} {
		if strings.HasSuffix(name, "-"+src) {
			return src
		}
	}
	return ""
}

func nameFrom(fm map[string]any, p string) string {
	aliases := asStringSlice(fm["aliases"])
	if len(aliases) > 0 {
		return strings.Trim(aliases[0], "\"")
	}
	// H1
	data, _ := os.ReadFile(p)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	base := strings.TrimSuffix(filepath.Base(p), ".md")
	// remove source suffix
	for _, src := range []string{"-xphb", "-xdmg", "-xmm", "-phb", "-dmg", "-mm"} {
		base = strings.TrimSuffix(base, src)
	}
	return strings.ReplaceAll(base, "-", " ")
}

func contentTypeFrom(p string, fm map[string]any) string {
	rel := p
	_ = rel
	// directory-based
	low := strings.ToLower(p)
	switch {
	case strings.Contains(low, "backgrounds"):
		return "background"
	case strings.Contains(low, "bestiary"):
		return "monster"
	case strings.Contains(low, "classes"):
		return "class"
	case strings.Contains(low, "decks"):
		return "deck"
	case strings.Contains(low, "feats"):
		return "feat"
	case strings.Contains(low, "items"):
		return "item"
	case strings.Contains(low, "optional-features"):
		return "optionalfeature"
	case strings.Contains(low, "races"):
		return "species"
	case strings.Contains(low, "spells"):
		return "spell"
	case strings.Contains(low, "tables"):
		return "table"
	case strings.Contains(low, "traps-hazards"):
		return "trap"
	case strings.Contains(low, "vehicles"):
		return "vehicle"
	case strings.Contains(low, "variant-rules"):
		return "variantrule"
	default:
		return "reference"
	}
}

func srdTypeFor(ct string) string {
	switch ct {
	case "class":
		return "class"
	case "species":
		return "race"
	case "background":
		return "background"
	case "feat":
		return "feat"
	case "spell":
		return "spell"
	case "monster":
		return "monster"
	case "optionalfeature":
		return "optionalfeature"
	case "variantrule":
		return "variantrule"
	case "item":
		return "item"
	case "table":
		return "table"
	case "trap":
		return "trap"
	case "deck":
		return "deck"
	case "vehicle":
		return "vehicle"
	default:
		return ct
	}
}

func deriveUID(rel string) string {
	h := sha256.Sum256([]byte(rel))
	// 26 hex chars
	s := strings.ToUpper(hex.EncodeToString(h[:]))
	if len(s) > 26 {
		s = s[:26]
	}
	return s
}

func cloneMap(m map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range m {
		out[k] = v
	}
	return out
}

// ---------- JSON entity loading ----------

func loadEntities(root string) map[string]*entity {
	reg := map[string]*entity{}
	dataDir := filepath.Join(root, "data")

	// helper to add
	add := func(typ, name, source string, data map[string]any, raw map[string]any) {
		key := typ + "|" + strings.ToLower(strings.TrimSpace(name)) + "|" + strings.ToLower(strings.TrimSpace(source))
		reg[key] = &entity{Type: typ, Name: name, Source: source, Data: data, Converted: countExpressionsInMap(raw)}
	}

	// monsters
	loadJSONFiles(filepath.Join(dataDir, "bestiary"), "bestiary-*.json", func(d map[string]any) {
		for _, m := range asObjSlice(d["monster"]) {
			src := str(m["source"])
			if !isSRDSrc(src) {
				// still load all; filter later by usage
			}
			data := extractMonster(m)
			add("monster", str(m["name"]), src, data, m)
		}
	})
	// spells
	loadJSONFiles(filepath.Join(dataDir, "spells"), "spells-*.json", func(d map[string]any) {
		for _, s := range asObjSlice(d["spell"]) {
			add("spell", str(s["name"]), str(s["source"]), extractSpell(s), s)
		}
	})
	// classes
	loadJSONFiles(filepath.Join(dataDir, "class"), "class-*.json", func(d map[string]any) {
		for _, c := range asObjSlice(d["class"]) {
			if str(c["source"]) == "" {
				continue
			}
			// only XPHB/P... load all
			add("class", str(c["name"]), str(c["source"]), extractClass(c), c)
		}
	})
	// races
	loadSingle(filepath.Join(dataDir, "races.json"), func(d map[string]any) {
		for _, r := range asObjSlice(d["race"]) {
			add("species", str(r["name"]), str(r["source"]), extractRace(r), r)
		}
	})
	// backgrounds
	loadSingle(filepath.Join(dataDir, "backgrounds.json"), func(d map[string]any) {
		for _, b := range asObjSlice(d["background"]) {
			add("background", str(b["name"]), str(b["source"]), extractBackground(b), b)
		}
	})
	// feats
	loadSingle(filepath.Join(dataDir, "feats.json"), func(d map[string]any) {
		for _, f := range asObjSlice(d["feat"]) {
			add("feat", str(f["name"]), str(f["source"]), extractFeat(f), f)
		}
	})
	// optional features
	loadSingle(filepath.Join(dataDir, "optionalfeatures.json"), func(d map[string]any) {
		for _, o := range asObjSlice(d["optionalfeature"]) {
			add("optionalfeature", str(o["name"]), str(o["source"]), extractOptional(o), o)
		}
	})
	// variantrules
	loadSingle(filepath.Join(dataDir, "variantrules.json"), func(d map[string]any) {
		for _, v := range asObjSlice(d["variantrule"]) {
			add("variantrule", str(v["name"]), str(v["source"]), extractVariant(v), v)
		}
	})
	// items
	loadSingle(filepath.Join(dataDir, "items.json"), func(d map[string]any) {
		for _, it := range asObjSlice(d["item"]) {
			add("item", str(it["name"]), str(it["source"]), extractItem(it), it)
		}
	})
	// traps/hazards
	loadSingle(filepath.Join(dataDir, "trapshazards.json"), func(d map[string]any) {
		for _, t := range asObjSlice(d["trap"]) {
			add("trap", str(t["name"]), str(t["source"]), extractTrap(t), t)
		}
		for _, h := range asObjSlice(d["hazard"]) {
			add("trap", str(h["name"]), str(h["source"]), extractTrap(h), h)
		}
	})
	// decks
	loadSingle(filepath.Join(dataDir, "decks.json"), func(d map[string]any) {
		for _, dk := range asObjSlice(d["deck"]) {
			add("deck", str(dk["name"]), str(dk["source"]), map[string]any{}, dk)
		}
	})
	// vehicles
	loadSingle(filepath.Join(dataDir, "vehicles.json"), func(d map[string]any) {
		for _, v := range asObjSlice(d["vehicle"]) {
			add("vehicle", str(v["name"]), str(v["source"]), map[string]any{}, v)
		}
	})
	// tables
	loadSingle(filepath.Join(dataDir, "tables.json"), func(d map[string]any) {
		for _, t := range asObjSlice(d["table"]) {
			add("table", str(t["name"]), str(t["source"]), map[string]any{}, t)
		}
	})
	return reg
}

func isSRDSrc(s string) bool {
	l := strings.ToLower(s)
	return l == "xphb" || l == "xdmg" || l == "xmm"
}

func loadJSONFiles(dir, pattern string, fn func(map[string]any)) {
	matches, _ := filepath.Glob(filepath.Join(dir, pattern))
	for _, f := range matches {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var d map[string]any
		if json.Unmarshal(b, &d) != nil {
			continue
		}
		fn(d)
	}
}

func loadSingle(path string, fn func(map[string]any)) {
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var d map[string]any
	if json.Unmarshal(b, &d) != nil {
		return
	}
	fn(d)
}

func asAnySlice(v any) []any {
	if arr, ok := v.([]any); ok {
		return arr
	}
	return nil
}

func asObjSlice(v any) []map[string]any {
	var out []map[string]any
	arr, ok := v.([]any)
	if !ok {
		return out
	}
	for _, e := range arr {
		if m, ok := e.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func str(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case float64:
		// integer-valued
		if t == float64(int(t)) {
			return strconv.Itoa(int(t))
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	}
	return fmt.Sprint(v)
}

func extractMonster(m map[string]any) map[string]any {
	data := map[string]any{}
	// size
	if sz := asStringSlice(m["size"]); len(sz) > 0 {
		data["size"] = sizeName(sz[0])
	}
	// type
	if t := m["type"]; t != nil {
		switch v := t.(type) {
		case string:
			data["type"] = v
		case map[string]any:
			if c, ok := v["choose"]; ok {
				data["type"] = str(c)
			} else if tt, ok := v["type"]; ok {
				switch t2 := tt.(type) {
				case map[string]any:
					if c2, ok := t2["choose"]; ok {
						data["type"] = str(c2)
					} else {
						data["type"] = str(t2)
					}
				default:
					data["type"] = str(tt)
				}
			} else {
				data["type"] = str(v)
			}
		}
	}
	// alignment
	if al := asStringSlice(m["alignment"]); len(al) > 0 {
		data["alignment"] = alignmentNames(al)
	}
	// ac
	if ac := m["ac"]; ac != nil {
		switch v := ac.(type) {
		case float64:
			data["ac"] = int(v)
		case []any:
			if len(v) > 0 {
				if n, ok := v[0].(float64); ok {
					data["ac"] = int(n)
				} else if mm, ok := v[0].(map[string]any); ok {
					if n, ok := mm["ac"].(float64); ok {
						data["ac"] = int(n)
					}
				}
			}
		}
	}
	// hp
	if hp, ok := m["hp"].(map[string]any); ok {
		if avg, ok := hp["average"].(float64); ok {
			data["hp"] = int(avg)
		}
	}
	// speed
	if sp, ok := m["speed"].(map[string]any); ok {
		data["speed"] = sp
	} else if n, ok := m["speed"].(float64); ok {
		data["speed"] = int(n)
	}
	// abilities
	abilities := map[string]any{}
	for _, k := range []string{"str", "dex", "con", "int", "wis", "cha"} {
		if v, ok := m[k].(float64); ok {
			abilities[k] = int(v)
		}
	}
	if len(abilities) > 0 {
		data["abilities"] = abilities
	}
	// passive
	if p, ok := m["passive"].(float64); ok {
		data["passive_perception"] = int(p)
	}
	// cr
	if cr := m["cr"]; cr != nil {
		switch v := cr.(type) {
		case map[string]any:
			data["cr"] = str(v["cr"])
			data["cr_xp"] = crXP(str(v["cr"]))
		default:
			data["cr"] = str(cr)
			data["cr_xp"] = crXP(str(cr))
		}
	}
	// immune/resist/vulnerable
	data["immune"] = strSlice(m["immune"])
	data["resist"] = strSlice(m["resist"])
	data["vulnerable"] = strSlice(m["vulnerable"])
	data["condition_immunities"] = strSlice(m["conditionImmune"])
	// languages
	data["languages"] = asStringSlice(m["languages"])
	// senses (empty list default)
	data["senses"] = []any{}
	return data
}

func extractSpell(s map[string]any) map[string]any {
	data := map[string]any{}
	data["level"] = intOr(s["level"], 0)
	data["school"] = schoolName(str(s["school"]))
	// casting time
	if t, ok := s["time"].([]any); ok && len(t) > 0 {
		if tm, ok := t[0].(map[string]any); ok {
			data["casting_time"] = fmt.Sprintf("%v %v", tm["number"], tm["unit"])
		}
	}
	// range
	if r, ok := s["range"].(map[string]any); ok {
		data["range"] = rangeStr(r)
	}
	// components
	data["components"] = componentsStr(s["components"])
	// duration
	if dur, ok := s["duration"].([]any); ok && len(dur) > 0 {
		if dm, ok := dur[0].(map[string]any); ok {
			data["duration"] = durationStr(dm)
		}
	}
	// classes from tags? leave empty; pipeline fills from markdown tags later? We'll set empty list
	data["classes"] = []any{}
	return data
}

func extractClass(c map[string]any) map[string]any {
	data := map[string]any{}
	if hd, ok := c["hd"].(map[string]any); ok {
		if n, ok := hd["faces"].(float64); ok {
			data["hit_die"] = int(n)
		}
	}
	// saving throws from proficiency
	if prof, ok := c["proficiency"].([]any); ok {
		var saves []any
		for _, p := range prof {
			saves = append(saves, p)
		}
		data["saving_throws"] = saves
	}
	// primary abilities: proficiency (same for 5e)
	data["primary_abilities"] = asStringSlice(c["proficiency"])
	// proficiencies
	if sp, ok := c["startingProficiencies"].(map[string]any); ok {
		data["proficiencies"] = sp
	}
	// features from classFeatures
	var features []any
	for _, raw := range asAnySlice(c["classFeatures"]) {
		var cf string
		switch v := raw.(type) {
		case string:
			cf = v
		case map[string]any:
			if s, ok := v["classFeature"].(string); ok {
				cf = s
			} else {
				continue
			}
		default:
			continue
		}
		// format "Name|Class||Level" or "Name|Class||Level|Source"
		parts := strings.Split(cf, "|")
		name := parts[0]
		level := 0
		if len(parts) >= 4 {
			level, _ = strconv.Atoi(strings.TrimSpace(parts[3]))
		}
		if strings.Contains(name, "gainSubclassFeature") {
			continue
		}
		if name == "" {
			continue
		}
		features = append(features, map[string]any{"name": name, "level": level})
	}
	// dedupe? keep
	data["features"] = features
	data["spellcasting_ability"] = ""
	data["caster_progression"] = ""
	return data
}

func extractRace(r map[string]any) map[string]any {
	data := map[string]any{}
	// size
	if sz := asStringSlice(r["size"]); len(sz) > 0 {
		data["size"] = sizeName(sz[0])
	}
	// speed
	if sp, ok := r["speed"].(float64); ok {
		data["speed"] = int(sp)
	} else {
		data["speed"] = r["speed"]
	}
	data["ability_bonuses"] = []any{}
	// languages
	data["languages"] = asStringSlice(r["languages"])
	// traits: entries names
	var traits []any
	if entries, ok := r["entries"].([]any); ok {
		for _, e := range entries {
			if em, ok := e.(map[string]any); ok {
				if n, ok := em["name"].(string); ok {
					traits = append(traits, n)
				}
			}
		}
	}
	data["traits"] = traits
	return data
}

func extractBackground(b map[string]any) map[string]any {
	data := map[string]any{}
	data["ability_choices"] = b["ability"]
	// skill proficiencies
	var skills []any
	if sp, ok := b["skillProficiencies"].([]any); ok {
		for _, s := range sp {
			if m, ok := s.(map[string]any); ok {
				var keys []string
				for k := range m {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				for _, k := range keys {
					skills = append(skills, k)
				}
			}
		}
	}
	data["skill_proficiencies"] = skills
	var tools []any
	if tp, ok := b["toolProficiencies"].([]any); ok {
		for _, t := range tp {
			if m, ok := t.(map[string]any); ok {
				var keys []string
				for k := range m {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				for _, k := range keys {
					tools = append(tools, k)
				}
			}
		}
	}
	data["tool_proficiencies"] = tools
	data["starting_equipment"] = b["startingEquipment"]
	// feats
	var feats []any
	if fts, ok := b["feats"].([]any); ok {
		for _, f := range fts {
			if m, ok := f.(map[string]any); ok {
				var keys []string
				for k := range m {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				for _, k := range keys {
					feats = append(feats, k)
				}
			}
		}
	}
	data["feats"] = feats
	return data
}

func extractFeat(f map[string]any) map[string]any {
	data := map[string]any{}
	data["category"] = str(f["category"])
	data["prerequisites"] = f["prerequisite"]
	data["ability"] = f["ability"]
	return data
}

func extractOptional(o map[string]any) map[string]any {
	data := map[string]any{}
	data["featureType"] = o["featureType"]
	data["prerequisites"] = o["prerequisite"]
	return data
}

func extractVariant(v map[string]any) map[string]any {
	data := map[string]any{}
	data["rule_type"] = str(v["ruleType"])
	data["rule"] = str(v["ruleType"])
	return data
}

func extractItem(it map[string]any) map[string]any {
	data := map[string]any{}
	data["type"] = str(it["type"])
	data["rarity"] = str(it["rarity"])
	data["reqAttune"] = str(it["reqAttune"])
	return data
}

func extractTrap(t map[string]any) map[string]any {
	data := map[string]any{}
	data["type"] = str(t["type"])
	return data
}

func sizeName(code string) string {
	switch strings.ToUpper(code) {
	case "T":
		return "Tiny"
	case "S":
		return "Small"
	case "M":
		return "Medium"
	case "L":
		return "Large"
	case "H":
		return "Huge"
	case "G":
		return "Gargantuan"
	}
	return code
}

func alignmentNames(codes []string) []any {
	// codes like "L","N","C","G","E","U" or combinations
	out := []any{}
	// If single combined? just join
	if len(codes) == 1 {
		out = append(out, codes[0])
		return out
	}
	// map pairs
	names := map[string]string{"L": "Lawful", "N": "Neutral", "C": "Chaotic", "G": "Good", "E": "Evil", "U": "Unaligned"}
	var parts []string
	for _, c := range codes {
		if n, ok := names[strings.ToUpper(c)]; ok {
			parts = append(parts, n)
		} else {
			parts = append(parts, c)
		}
	}
	out = append(out, strings.Join(parts, " "))
	return out
}

func schoolName(code string) string {
	switch strings.ToUpper(code) {
	case "A":
		return "Abjuration"
	case "C":
		return "Conjuration"
	case "D":
		return "Divination"
	case "E":
		return "Enchantment"
	case "V":
		return "Evocation"
	case "I":
		return "Illusion"
	case "N":
		return "Necromancy"
	case "T":
		return "Transmutation"
	}
	return code
}

func rangeStr(r map[string]any) string {
	if d, ok := r["distance"].(map[string]any); ok {
		return fmt.Sprintf("%v %v", d["amount"], d["type"])
	}
	return str(r["type"])
}

func componentsStr(c any) string {
	m, ok := c.(map[string]any)
	if !ok {
		return str(c)
	}
	var parts []string
	if b, _ := m["v"].(bool); b {
		parts = append(parts, "V")
	}
	if b, _ := m["s"].(bool); b {
		parts = append(parts, "S")
	}
	if mm, ok := m["m"]; ok {
		if ms, ok := mm.(map[string]any); ok {
			parts = append(parts, "M ("+str(ms["text"])+")")
		} else if ms, ok := mm.(string); ok {
			parts = append(parts, "M ("+ms+")")
		} else {
			parts = append(parts, "M")
		}
	}
	return strings.Join(parts, ", ")
}

func durationStr(d map[string]any) string {
	if t, ok := d["type"].(string); ok {
		if t == "instant" {
			return "Instantaneous"
		}
		return t
	}
	return str(d)
}

func strSlice(v any) []any {
	arr, ok := v.([]any)
	if !ok {
		return []any{}
	}
	out := []any{}
	for _, e := range arr {
		out = append(out, e)
	}
	return out
}

func intOr(v any, def int) int {
	if n, ok := v.(float64); ok {
		return int(n)
	}
	return def
}

func crXP(cr string) int {
	// standard 5e XP by CR
	table := map[string]int{
		"0": 10, "1/8": 25, "1/4": 50, "1/2": 100,
		"1": 200, "2": 450, "3": 700, "4": 1100, "5": 1800,
		"6": 2300, "7": 2900, "8": 3900, "9": 5000, "10": 5900,
		"11": 7200, "12": 8400, "13": 10000, "14": 11500, "15": 13000,
		"16": 15000, "17": 18000, "18": 20000, "19": 22000, "20": 25000,
		"21": 33000, "22": 41000, "23": 50000, "24": 62000, "25": 75000,
		"26": 90000, "27": 105000, "28": 120000, "29": 135000, "30": 155000,
	}
	if x, ok := table[cr]; ok {
		return x
	}
	return 0
}

func countExpressionsInMap(m map[string]any) int {
	return countExpressionsInValue(m)
}

func countExpressionsInValue(v any) int {
	n := 0
	switch t := v.(type) {
	case string:
		n += countExpressions(t)
	case map[string]any:
		for _, val := range t {
			n += countExpressionsInValue(val)
		}
	case []any:
		for _, val := range t {
			n += countExpressionsInValue(val)
		}
	}
	return n
}

func countExpressions(s string) int {
	// Count upstream 5e-tools formula/display expressions (<$...$>).
	n := 0
	i := 0
	for {
		a := strings.Index(s[i:], "<$")
		if a < 0 {
			break
		}
		a += i
		b := strings.Index(s[a:], "$>")
		if b < 0 {
			break
		}
		b += a
		n++
		i = b + 2
	}
	return n
}

func compareDirs(a, b string) error {
	// walk a, compare each file to b
	var files []string
	filepath.WalkDir(a, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel(a, p)
			files = append(files, rel)
		}
		return nil
	})
	// also ensure b has same set
	var bfiles []string
	filepath.WalkDir(b, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel(b, p)
			bfiles = append(bfiles, rel)
		}
		return nil
	})
	sort.Strings(files)
	sort.Strings(bfiles)
	if len(files) != len(bfiles) {
		return fmt.Errorf("file count differs: %d vs %d", len(files), len(bfiles))
	}
	for i := range files {
		if files[i] != bfiles[i] {
			return fmt.Errorf("file set differs at %s vs %s", files[i], bfiles[i])
		}
	}
	for _, rel := range files {
		ba, _ := os.ReadFile(filepath.Join(a, rel))
		bb, _ := os.ReadFile(filepath.Join(b, rel))
		if !bytes.Equal(ba, bb) {
			return fmt.Errorf("first differing file: %s", rel)
		}
	}
	return nil
}

func writeArchive(srcDir, zipPath string) error {
	f, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	defer zw.Close()

	// demitable-import.json at root
	manifest := map[string]any{
		"id":          "dnd5e-srd",
		"name":        "D&D 5e SRD 5.2.1 Content",
		"version":     "1.0.0",
		"kind":        "content",
		"target":      "Notes/Content",
		"requires":    map[string]any{"packs": []string{"dnd5e"}, "app": ">=0.1.0"},
		"license":     "CC-BY-4.0",
		"attribution": "Wizards of the Coast — D&D System Reference Document 5.2.1",
	}
	mb, _ := json.MarshalIndent(manifest, "", "  ")
	mb = append(mb, '\n')
	mw, err := zw.Create("demitable-import.json")
	if err != nil {
		return err
	}
	if _, err := mw.Write(mb); err != nil {
		return err
	}

	// Walk srcDir and add files under vault root (preserve relative paths).
	return filepath.WalkDir(srcDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(srcDir, p)
		rel = filepath.ToSlash(rel)
		w, err := zw.Create(rel)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	})
}
