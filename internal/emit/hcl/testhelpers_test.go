package hcl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/autonation/autonation/internal/catalog"
	"github.com/autonation/autonation/internal/ir"
	"github.com/autonation/autonation/internal/resolver"
)

// testCatalog loads the real catalog data from the repo root.
//
// The path depends on how the test binary is invoked: `go test` runs from the
// package directory while a compiled binary runs from wherever it is launched,
// so both layouts are probed.
func testCatalog(t *testing.T) *catalog.Catalog {
	t.Helper()
	for _, dir := range []string{
		"../../../catalog",    // go test from internal/emit/hcl
		"../../../../catalog", // go test from a deeper package dir
		"catalog",             // compiled binary run from the repo root
		"../catalog",          // compiled binary run from a subdirectory
		"../../catalog",       // compiled binary run from internal/
	} {
		if _, err := os.Stat(dir); err == nil {
			c, err := catalog.Load(dir)
			if err != nil {
				t.Fatalf("catalog.Load %s: %v", dir, err)
			}
			return c
		}
	}
	t.Fatal("catalog data not found")
	return nil
}

// testPlan resolves a minimal three-resource blueprint.
func testPlan(t *testing.T, cat *catalog.Catalog) *resolver.Plan {
	t.Helper()
	bp := ir.NewBlueprint()
	bp.Metadata = ir.Metadata{
		BusinessUnit: "test",
		Platform:     "platform",
		Environment:  "Production",
		Location:     "Southeast Asia",
		Tags:         map[string]string{"Environment": "Production", "Owner": "team"},
	}
	bp.AddComponent(&ir.Component{ID: "hub", Kind: "resource_group", Stack: "Core"})
	bp.AddComponent(&ir.Component{ID: "spokevnet", Kind: "vnet", Stack: "Network"})
	bp.AddComponent(&ir.Component{ID: "aml", Kind: "subnet", Stack: "Network"})

	plan, err := resolver.New(cat).Resolve(bp)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	return plan
}

// baseName returns the final path element.
func baseName(path string) string {
	return filepath.Base(filepath.FromSlash(path))
}

// dirName returns the directory portion of a slash path.
func dirName(path string) string {
	return slashDir(filepath.Dir(filepath.FromSlash(path)))
}

// slashDir converts a native directory to a forward-slash path.
func slashDir(p string) string {
	return strings.ReplaceAll(p, string(os.PathSeparator), "/")
}

// splitLines splits content on newlines.
func splitLines(s string) []string {
	return strings.Split(s, "\n")
}

// trimSpace removes surrounding whitespace.
func trimSpace(s string) string {
	return strings.TrimSpace(s)
}

// contains reports whether substr is in s.
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// startsWith reports whether s has the given prefix.
func startsWith(s, prefix string) bool {
	return strings.HasPrefix(s, prefix)
}

// cut splits s on the first sep.
func cut(s, sep string) (string, string, bool) {
	return strings.Cut(s, sep)
}
