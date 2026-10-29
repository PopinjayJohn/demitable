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
	re := regexp.MustCompile(`m\.(HandleFunc|Handle)\(\s*"([^"]+)"`)
	seen := map[string]bool{}
	var out []openAPIRoute
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		for _, m := range re.FindAllStringSubmatch(string(b), -1) {
			pat := strings.TrimSpace(m[2])
			if pat == "" || strings.Contains(pat, "+") {
				continue
			}
			parts := strings.Fields(pat)
			if len(parts) != 2 {
				continue
			}
			method := strings.ToUpper(parts[0])
			path := parts[1]
			key := method + " " + path
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, openAPIRoute{Method: method, Path: path})
		}
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
