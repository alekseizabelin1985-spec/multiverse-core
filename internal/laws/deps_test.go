package laws

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const module = "multiverse-core.io/"

// TestLawsSeeNoOtherContext is `go list -deps ./internal/laws/...` of the DoD
// without the go command: the packages of this module the laws reach, through
// every import of their shipped files, contain no internal/* but the laws
// themselves. Above all not internal/swarm, whose WorldView the laws read
// through WorldVersions instead (depguard internal-laws, decision 1f). The
// walk reads the sources, so the answer depends neither on a module cache nor
// on the go binary being on the PATH of the test.
func TestLawsSeeNoOtherContext(t *testing.T) {
	deps := moduleDeps(t, module+"internal/laws")
	if !slices.Contains(deps, module+"shared/eventbus") {
		t.Fatalf("the walk found %v; it does not follow the imports", deps)
	}
	for _, dep := range deps {
		if strings.HasPrefix(dep, module+"internal/") && dep != module+"internal/laws" {
			t.Errorf("internal/laws depends on %s", dep)
		}
	}
}

// moduleDeps returns the packages of this module that root reaches, root
// included, sorted.
func moduleDeps(t *testing.T, root string) []string {
	t.Helper()
	seen := map[string]bool{}
	queue := []string{root}
	for len(queue) > 0 {
		pkg := queue[0]
		queue = queue[1:]
		if seen[pkg] {
			continue
		}
		seen[pkg] = true
		for _, imp := range shippedImports(t, pkg) {
			if strings.HasPrefix(imp, module) && !seen[imp] {
				queue = append(queue, imp)
			}
		}
	}
	out := make([]string, 0, len(seen))
	for pkg := range seen {
		out = append(out, pkg)
	}
	slices.Sort(out)
	return out
}

// shippedImports reads the imports of the non-test Go files of a package of
// this module. The directory comes from the import path: the package directory
// is two levels below the root of the module.
func shippedImports(t *testing.T, importPath string) []string {
	t.Helper()
	dir := filepath.Join("..", "..", filepath.FromSlash(strings.TrimPrefix(importPath, module)))
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	fset := token.NewFileSet()
	var out []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, imp := range file.Imports {
			out = append(out, strings.Trim(imp.Path.Value, `"`))
		}
	}
	return out
}
