package main

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
)

type openAPIRoute struct {
	Method string
	Path   string
}

var (
	directRouteRe   = regexp.MustCompile(`m\.(HandleFunc|Handle)\(\s*"([^"]+)"`)
	fullPathRouteRe = regexp.MustCompile(`"(GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS)\s+(/[^"]*)"`)
	quoteRe         = regexp.MustCompile(`"[^"]*"`)
	namedSliceRe    = regexp.MustCompile(`(\w+)\s*:?=\s*\[\]string\s*\{`)
	forRangeRe      = regexp.MustCompile(`for\s+_,\s*(\w+)\s*:=\s*range\s+(.*)$`)
	methodPathRe    = regexp.MustCompile(`^(GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS)\s+/`)
)

// hasClosingBraceOutsideQuotes reports whether s contains a '}' that is not
// inside a double-quoted string. Route path literals contain '{' (e.g.
// {uid}) but never a top-level closing brace outside a string.
func hasClosingBraceOutsideQuotes(s string) bool {
	return strings.Contains(quoteRe.ReplaceAllString(s, ""), "}")
}

// parseNamedSlices collects `name := []string{ ... }` (or `=`) declarations
// and returns the path/full-pattern string elements of each, keyed by name.
func parseNamedSlices(content string) map[string][]string {
	lines := strings.Split(content, "\n")
	named := map[string][]string{}
	for i := 0; i < len(lines); i++ {
		m := namedSliceRe.FindStringSubmatch(lines[i])
		if m == nil {
			continue
		}
		name := m[1]
		var strs []string
		j := i
		for ; j < len(lines); j++ {
			for _, sm := range quoteRe.FindAllStringSubmatch(lines[j], -1) {
				val := strings.Trim(sm[0], `"`)
				if strings.HasPrefix(val, "/") || methodPathRe.MatchString(val) {
					strs = append(strs, val)
				}
			}
			if j > i && hasClosingBraceOutsideQuotes(lines[j]) {
				break
			}
			if j == i && hasClosingBraceOutsideQuotes(lines[i]) {
				break
			}
		}
		if len(strs) > 0 {
			named[name] = strs
		}
		i = j
	}
	return named
}

// collectLoopRoutes extracts routes registered through loops over string
// slices, including slices whose elements are bare paths combined with a
// method prefix on the m.HandleFunc call (e.g. `m.HandleFunc("GET "+p, ...)`).
func collectLoopRoutes(content string, named map[string][]string, add func(string, string)) {
	lines := strings.Split(content, "\n")
	// find for-range header lines
	var headers []int
	for i, l := range lines {
		if forRangeRe.MatchString(l) {
			headers = append(headers, i)
		}
	}
	for hi, start := range headers {
		m := forRangeRe.FindStringSubmatch(lines[start])
		if m == nil {
			continue
		}
		loopVar := m[1]
		sliceExpr := strings.TrimSpace(m[2])
		inline := strings.Contains(sliceExpr, "[]string{")
		sliceIdent := sliceExpr
		if !inline {
			sliceIdent = strings.TrimSpace(strings.TrimSuffix(sliceIdent, "{"))
		}

		// collect slice string elements
		var strs []string
		if inline {
			j := start
			for ; j < len(lines); j++ {
				for _, sm := range quoteRe.FindAllStringSubmatch(lines[j], -1) {
					val := strings.Trim(sm[0], `"`)
					if strings.HasPrefix(val, "/") || methodPathRe.MatchString(val) {
						strs = append(strs, val)
					}
				}
				if j > start && hasClosingBraceOutsideQuotes(lines[j]) {
					break
				}
				if j == start && hasClosingBraceOutsideQuotes(lines[start]) {
					break
				}
			}
		} else {
			if v, ok := named[sliceIdent]; ok {
				strs = v
			}
		}

		// method prefixes used with this loop variable in the following block
		end := len(lines)
		if hi+1 < len(headers) {
			end = headers[hi+1]
		}
		methods := map[string]bool{}
		for j := start; j < end; j++ {
			mm := regexp.MustCompile(`m\.(HandleFunc|Handle)\(\s*"([A-Z]+)\s*"\s*\+\s*` + regexp.QuoteMeta(loopVar) + `\b`).FindStringSubmatch(lines[j])
			if mm != nil {
				methods[mm[2]] = true
			}
		}

		if len(strs) == 0 || len(methods) == 0 {
			continue
		}
		for _, p := range strs {
			if !strings.HasPrefix(p, "/") {
				continue
			}
			for meth := range methods {
				add(meth, p)
			}
		}
	}
}

// collectRegisteredRoutes statically scans the Go sources for
// `m.HandleFunc`/`m.Handle` calls whose pattern is a string literal of the
// form "METHOD path", plus routes registered through loops over string slices
// (including path slices combined with a method prefix), so that alias
// matrices and endpoints such as the verdicts and transcript-review paths
// are represented in the generated document.
func collectRegisteredRoutes() []openAPIRoute {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		file = "openapi.go"
	}
	dir := filepath.Dir(file)
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var out []openAPIRoute
	add := func(method, path string) {
		method = strings.ToUpper(strings.TrimSpace(method))
		path = strings.TrimSpace(path)
		if method == "" || path == "" || !strings.HasPrefix(path, "/") {
			return
		}
		key := method + " " + path
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, openAPIRoute{Method: method, Path: path})
	}

	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		content := string(b)

		// Direct registrations: m.HandleFunc("METHOD path", ...).
		for _, m := range directRouteRe.FindAllStringSubmatch(content, -1) {
			pat := strings.TrimSpace(m[2])
			if pat == "" || strings.Contains(pat, "+") {
				continue
			}
			parts := strings.Fields(pat)
			if len(parts) != 2 {
				continue
			}
			add(parts[0], parts[1])
		}

		// Full "METHOD /path" literals anywhere (covers slices whose
		// elements are complete patterns, e.g. move/token alias matrices).
		for _, m := range fullPathRouteRe.FindAllStringSubmatch(content, -1) {
			add(m[1], m[2])
		}

		// Loop-registered path slices.
		named := parseNamedSlices(content)
		collectLoopRoutes(content, named, add)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Method < out[j].Method
	})
	return out
}

func generateOpenAPI() map[string]any {
	paths := map[string]any{}
	for _, rt := range collectRegisteredRoutes() {
		p, ok := paths[rt.Path].(map[string]any)
		if !ok {
			p = map[string]any{}
			paths[rt.Path] = p
		}
		p[strings.ToLower(rt.Method)] = map[string]any{
			"responses": map[string]any{
				"200": map[string]any{"description": "OK"},
			},
		}
	}
	return map[string]any{
		"openapi": "3.0.3",
		"info": map[string]any{
			"title":   "Demitable",
			"version": "1.0.0",
		},
		"paths": paths,
	}
}

func OpenAPIDocument() map[string]any { return generateOpenAPI() }
func OpenAPISpec() map[string]any     { return generateOpenAPI() }
