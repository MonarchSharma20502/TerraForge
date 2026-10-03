package hcl

import (
	"testing"
)

// TestEmitSmoke checks the emitter produces the six-file stack set and the
// module blueprints for a minimal plan.
func TestEmitSmoke(t *testing.T) {
	cat := testCatalog(t)
	plan := testPlan(t, cat)

	e := New(cat)
	files, err := e.Emit(plan)
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if len(files.Files) == 0 {
		t.Fatal("Emit produced no files")
	}

	// Every Workload stack must have exactly these six files.
	want := map[string]bool{
		"main.tf": false, "locals.tf": false, "variables.tf": false,
		"data.tf": false, "provider.tf": false, "terraform.tfvars": false,
	}
	for stack := range plan.Stacks {
		for _, f := range files.Files {
			got := baseName(f.Path)
			if dirName(f.Path) == "Workload/"+stack {
				if _, ok := want[got]; !ok {
					t.Errorf("stack %s: unexpected file %s", stack, f.Path)
				}
				want[got] = true
			}
		}
		for name, seen := range want {
			if !seen {
				t.Errorf("stack %s: missing %s", stack, name)
			}
		}
	}

	// main.tf must contain module calls only: every argument resolves to
	// local.* or var.*. This is the user's rule 3.
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
			// A module source path is a literal by nature: it points at a
			// directory, not a computed value. The reference repos use relative
			// paths and the plan keeps that. Everything else must be a
			// reference.
			if key == "source" && startsWith(value, `"../`) {
				continue
			}
			if !startsWith(value, "local.") && !startsWith(value, "var.") {
				t.Errorf("%s: hardcoded literal in main.tf: %s", f.Path, line)
			}
		}
	}
}

// TestProviderNoBackend checks provider.tf carries no backend block and no secret.
func TestProviderNoBackend(t *testing.T) {
	cat := testCatalog(t)
	plan := testPlan(t, cat)

	files, err := New(cat).Emit(plan)
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	for _, f := range files.Files {
		if baseName(f.Path) != "provider.tf" {
			continue
		}
		if contains(f.Content, "backend") {
			t.Errorf("%s: backend block present", f.Path)
		}
		for _, secret := range []string{"access_key", "connection_string", "sas_token"} {
			if contains(f.Content, secret) {
				t.Errorf("%s: secret-shaped string %q", f.Path, secret)
			}
		}
	}
}
