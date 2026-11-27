// Package openapi_test keeps the written contract (openapi.yaml) and the routes the services register from drifting
// apart. Structure and references are validated in CI with Redocly.
package openapi_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// Operational endpoints, not part of the product API.
var operational = map[string]bool{"/": true, "/healthz": true, "/readyz": true, "/metrics": true, "/csp-report": true}

var methods = map[string]bool{"get": true, "put": true, "post": true, "delete": true, "patch": true}

var parameter = regexp.MustCompile(`\{[^}]*\}`)

// normal drops the version prefix (the gateway exposes /api/v1/*, some services mount /v1/*) and parameter names.
func normal(path string) string {
	for _, prefix := range []string{"/api/v1", "/v1"} {
		if strings.HasPrefix(path, prefix+"/") {
			path = strings.TrimPrefix(path, prefix)
			break
		}
	}
	return parameter.ReplaceAllString(path, "{}")
}

// registered returns the "METHOD /path" routes with a method, and the prefixes of handlers that route by themselves.
func registered(t *testing.T) (routes map[string]bool, prefixes []string) {
	t.Helper()
	routes = map[string]bool{}
	fset := token.NewFileSet()
	for _, root := range []string{"../../cmd", "../../internal"} {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") ||
				strings.Contains(filepath.ToSlash(path), "fake") {
				return err
			}
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return err
			}
			ast.Inspect(file, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok || len(call.Args) != 2 {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || (sel.Sel.Name != "Handle" && sel.Sel.Name != "HandleFunc") {
					return true
				}
				lit, ok := call.Args[0].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				pattern, err := strconv.Unquote(lit.Value)
				if err != nil {
					return true
				}
				method, route, found := strings.Cut(pattern, " ")
				if !found {
					route = pattern
				}
				if !strings.HasPrefix(route, "/") || operational[route] || route == "/health" || route == "/api/v1/" {
					return true
				}
				if found {
					routes[method+" "+normal(route)] = true
				} else {
					prefixes = append(prefixes, normal(route))
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return routes, prefixes
}

func documented(t *testing.T) map[string]bool {
	t.Helper()
	raw, err := os.ReadFile("openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		Paths map[string]map[string]yaml.Node `yaml:"paths"`
	}
	if err := yaml.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("openapi.yaml does not parse: %v", err)
	}
	out := map[string]bool{}
	for path, item := range spec.Paths {
		for method := range item {
			if methods[method] {
				out[strings.ToUpper(method)+" "+normal(path)] = true
			}
		}
	}
	if len(out) == 0 {
		t.Fatal("openapi.yaml documents no operation")
	}
	return out
}

// listed reads a divergence list: one "METHOD /path" per line, # comments.
func listed(t *testing.T, name string) map[string]bool {
	t.Helper()
	raw, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, line := range strings.Split(string(raw), "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
			out[line] = true
		}
	}
	return out
}

func compare(t *testing.T, file, what string, found []string) {
	t.Helper()
	sort.Strings(found)
	if os.Getenv("UPDATE_CONTRACT_LISTS") == "1" {
		header := "# " + what + "\n# This list may only shrink. Regenerate: UPDATE_CONTRACT_LISTS=1 go test ./api/openapi/\n"
		if err := os.WriteFile(file, []byte(header+strings.Join(found, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	known := listed(t, file)
	for _, f := range found {
		if !known[f] {
			t.Errorf("%s: %s (update openapi.yaml)", what, f)
		}
		delete(known, f)
	}
	for f := range known {
		t.Errorf("%s no longer diverges: remove it from %s", f, file)
	}
}

func TestTheContractAndTheRoutesDoNotDrift(t *testing.T) {
	routes, prefixes := registered(t)
	spec := documented(t)
	if len(routes) < 50 {
		t.Fatalf("only %d routes found: the scan is broken", len(routes))
	}
	var undocumented, unrouted []string
	for r := range routes {
		if !spec[r] {
			undocumented = append(undocumented, r)
		}
	}
	for op := range spec {
		if routes[op] {
			continue
		}
		_, path, _ := strings.Cut(op, " ")
		covered := path == "/health"
		for _, p := range prefixes {
			if path == p || path == strings.TrimSuffix(p, "/") || (strings.HasSuffix(p, "/") && strings.HasPrefix(path, p)) {
				covered = true
			}
		}
		if !covered {
			unrouted = append(unrouted, op)
		}
	}
	compare(t, "undocumented-routes.txt", "Route without documentation", undocumented)
	compare(t, "unimplemented-operations.txt", "Documented operation no service registers", unrouted)
}
