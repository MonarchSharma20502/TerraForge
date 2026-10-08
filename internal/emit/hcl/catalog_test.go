package hcl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/autonation/autonation/internal/catalog"
	"github.com/autonation/autonation/internal/policy"
	"github.com/autonation/autonation/internal/resolver"
	"github.com/autonation/autonation/internal/spec"
)

// TestEveryKindGenerates is the catalog onboarding gate: every resource kind the
// catalog declares must parse, resolve, emit and clear the policy gate from a
// spec that instantiates it once.
//
// Adding a resource kind is a catalog data change plus a golden test; this is
// that test for the azurerm examples service inventory. It catches the three
// ways a new catalog entry breaks the pipeline: a socket that names a kind the
// resolver cannot supply, a required variable the stack cannot bind, and a
// generated name that violates the kind's own strict-name bounds.
func TestEveryKindGenerates(t *testing.T) {
	cat := testCatalog(t)
	repo := repoRoot(t)

	specDoc := buildAllKindsSpec(t, repo, cat)
	bp, err := spec.ParseDocument([]byte(specDoc))
	if err != nil {
		t.Fatalf("parse all-kinds spec: %v", err)
	}
	plan, err := resolver.New(cat).Resolve(bp)
	if err != nil {
		t.Fatalf("resolve all-kinds spec: %v", err)
	}
	files, err := New(cat).Emit(plan)
	if err != nil {
		t.Fatalf("emit all-kinds spec: %v", err)
	}

	if len(files.Files) == 0 {
		t.Fatal("emit produced no files")
	}

	// Every kind in the catalog must appear in the plan. A kind that never
	// reaches the plan would silently fall out of the generated tree.
	seen := map[string]bool{}
	for _, res := range plan.Ordered {
		seen[res.Component.Kind] = true
	}
	for _, kind := range cat.KindList() {
		if !seen[kind] {
			t.Errorf("kind %q did not reach the plan", kind)
		}
	}

	// Every Workload stack carries exactly the six files.
	for stack := range plan.Stacks {
		count := 0
		for _, f := range files.Files {
			if dirName(f.Path) == "Workload/"+stack {
				count++
				if !stackFiles[baseName(f.Path)] {
					t.Errorf("stack %s: unexpected file %s", stack, f.Path)
				}
			}
		}
		if count != len(stackFiles) {
			t.Errorf("stack %s: got %d files, want %d", stack, count, len(stackFiles))
		}
	}

	// main.tf holds module calls only: every argument resolves to local.* or
	// var.*. This is the user's rule 3, checked on the full catalog.
	for _, f := range files.Files {
		if baseName(f.Path) != "main.tf" || dirName(f.Path) == "Modules" {
			continue
		}
		for _, line := range splitLines(f.Content) {
			line = trimSpace(line)
			if line == "" || line[0] == '#' || !contains(line, "=") {
				continue
			}
			key, value, found := cut(line, "=")
			if !found {
				continue
			}
			key = trimSpace(key)
			value = trimSpace(value)
			if key == "source" && startsWith(value, `"../`) {
				continue
			}
			if !startsWith(value, "local.") && !startsWith(value, "var.") {
				t.Errorf("%s: hardcoded literal in main.tf: %s", f.Path, line)
			}
		}
	}

	// The policy gate must pass on the whole tree. A catalog entry that emits a
	// secret-shaped string or an out-of-bounds strict name lands here.
	fileMap := map[string]string{}
	for _, f := range files.Files {
		fileMap[f.Path] = f.Content
	}
	result := policy.New(cat).Check(plan, fileMap)
	if !result.Passed {
		for _, f := range result.Findings {
			t.Errorf("policy %s: %s", f.Rule, f.Message)
		}
	}
}

// stackFiles is the fixed six-file set every Workload stack carries.
var stackFiles = map[string]bool{
	"main.tf": true, "locals.tf": true, "variables.tf": true,
	"data.tf": true, "provider.tf": true, "terraform.tfvars": true,
}

// buildAllKindsSpec writes one instance of every catalog kind to a temp spec
// and returns its contents. Kinds whose required variables are other resource
// ids are still emitted: the resolver binds a socket to the single component of
// that kind when the spec declares exactly one, which is the case here.
func buildAllKindsSpec(t *testing.T, repo string, cat *catalog.Catalog) string {
	t.Helper()

	var sb strings.Builder
	sb.WriteString("apiVersion: autonation/v1\n")
	sb.WriteString("metadata:\n")
	sb.WriteString("  businessUnit: test\n")
	sb.WriteString("  platform: platform\n")
	sb.WriteString("  environment: Production\n")
	sb.WriteString("  location: Southeast Asia\n")
	sb.WriteString("  subscriptionId: 00000000-0000-0000-0000-000000000000\n")
	sb.WriteString("  owner: platform-team\n")
	sb.WriteString("  tags:\n")
	sb.WriteString("    Environment: Production\n")
	sb.WriteString("    Owner: platform-team\n")
	sb.WriteString("resources:\n")

	for _, kind := range cat.KindList() {
		entry, ok := cat.Get(kind)
		if !ok {
			continue
		}
		sb.WriteString("  " + kind + ":\n")
		sb.WriteString("    " + kind + "01: {}\n")
		_ = entry
	}

	specPath := filepath.Join(repo, "examples", "all_kinds.yaml")
	if err := os.WriteFile(specPath, []byte(sb.String()), 0o644); err != nil {
		t.Fatalf("write %s: %v", specPath, err)
	}
	t.Cleanup(func() {
		_ = os.Remove(specPath)
	})
	return sb.String()
}
