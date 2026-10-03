package hcl

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/autonation/autonation/internal/catalog"
	"github.com/autonation/autonation/internal/emit/diagram"
	"github.com/autonation/autonation/internal/policy"
	"github.com/autonation/autonation/internal/resolver"
	"github.com/autonation/autonation/internal/spec"
	"github.com/autonation/autonation/internal/workspace"
)

// generatorVersion mirrors cmd/autonation.Version so the golden manifest this
// test rebuilds matches the one the CLI writes.
const generatorVersion = "0.1.0"

// TestGoldenTree regenerates the example spec through the full pipeline and
// asserts the output is byte-identical to the committed golden tree.
//
// This is the regeneration contract: running generate twice on the same spec
// must be a zero-diff no-op, and any change to the emitter, resolver or catalog
// shows up here as a golden diff that has to be reviewed and re-seeded.
func TestGoldenTree(t *testing.T) {
	cat := testCatalog(t)
	repo := repoRoot(t)
	specPath := filepath.Join(repo, "examples", "vnet.yaml")
	goldenDir := filepath.Join(repo, "examples", "golden")

	got := generate(t, cat, specPath)

	// Every committed golden file must be reproduced byte for byte.
	missing := 0
	walkGolden(t, goldenDir, func(rel string, want string) {
		g, ok := got[rel]
		if !ok {
			t.Errorf("golden %s: not generated", rel)
			missing++
			return
		}
		if g != want {
			t.Errorf("golden %s: byte mismatch\n--- want ---\n%s\n--- got ---\n%s", rel, want, g)
		}
	})
	if missing > 0 {
		t.Fatalf("golden tree has %d file(s) the generator no longer produces", missing)
	}

	// And nothing extra: the generator must not emit files the golden tree does
	// not carry.
	for path := range got {
		if _, err := os.Stat(filepath.Join(goldenDir, filepath.FromSlash(path))); err != nil {
			t.Errorf("generated %s: not in the golden tree", path)
		}
	}
}

// TestRegenerationIsZeroDiff proves the manifest contract: emitting twice from
// the same spec yields identical bytes and identical per-file hashes.
func TestRegenerationIsZeroDiff(t *testing.T) {
	cat := testCatalog(t)
	repo := repoRoot(t)
	specPath := filepath.Join(repo, "examples", "vnet.yaml")

	first := generate(t, cat, specPath)
	second := generate(t, cat, specPath)

	if len(first) != len(second) {
		t.Fatalf("file count changed across regenerations: %d vs %d", len(first), len(second))
	}
	for path, content := range first {
		if second[path] != content {
			t.Errorf("regeneration diff at %s", path)
		}
	}
	if !mapsEqual(workspace.HashFiles(first), workspace.HashFiles(second)) {
		t.Error("manifest file hashes differ across regenerations")
	}
}

// TestGoldenPassesPolicyGate makes sure the committed golden tree clears the
// five v0.1 rules; a golden fixture that fails the gate would be a regression
// the download button must block.
func TestGoldenPassesPolicyGate(t *testing.T) {
	cat := testCatalog(t)
	repo := repoRoot(t)
	specPath := filepath.Join(repo, "examples", "vnet.yaml")

	raw, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read %s: %v", specPath, err)
	}
	bp, err := spec.ParseDocument(raw)
	if err != nil {
		t.Fatalf("parse %s: %v", specPath, err)
	}
	plan, err := resolver.New(cat).Resolve(bp)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	files := generate(t, cat, specPath)

	result := policy.New(cat).Check(plan, files)
	if !result.Passed {
		for _, f := range result.Findings {
			t.Errorf("policy %s: %s", f.Rule, f.Message)
		}
	}
}

// generate runs the full pipeline, exactly as the CLI does, and returns the
// emitted file map keyed by slash-separated path.
func generate(t *testing.T, cat *catalog.Catalog, specPath string) map[string]string {
	t.Helper()

	raw, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read %s: %v", specPath, err)
	}
	bp, err := spec.ParseDocument(raw)
	if err != nil {
		t.Fatalf("parse %s: %v", specPath, err)
	}
	plan, err := resolver.New(cat).Resolve(bp)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	files, err := New(cat).Emit(plan)
	if err != nil {
		t.Fatalf("emit: %v", err)
	}

	out := map[string]string{}
	for _, f := range files.Files {
		out[f.Path] = f.Content
	}

	diag := diagram.New(cat)
	out["architecture.mmd"] = diag.Mermaid(plan)
	out["architecture.d2"] = diag.D2(plan)

	manifest, err := yamlMarshalManifest(raw, out)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	out[".autonation.yaml"] = manifest

	return out
}

// yamlMarshalManifest builds the manifest body the way the workspace package
// writes it, so the golden manifest is reproduced byte for byte.
func yamlMarshalManifest(raw []byte, files map[string]string) (string, error) {
	m := &workspace.Manifest{
		APIVersion:       "autonation/v1",
		GeneratorVersion: generatorVersion,
		SpecHash:         workspace.HashSpec(raw),
		Files:            workspace.HashFiles(files),
	}
	data, err := workspace.MarshalManifest(m)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// walkGolden calls fn for every file under dir, with paths relative to dir and
// slash-separated.
func walkGolden(t *testing.T, dir string, fn func(rel string, content string)) {
	t.Helper()
	walkGoldenDir(t, dir, dir, fn)
}

// walkGoldenDir recurses under root, reporting paths relative to root.
func walkGoldenDir(t *testing.T, root, dir string, fn func(rel string, content string)) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read golden dir %s: %v", dir, err)
	}
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		if e.IsDir() {
			walkGoldenDir(t, root, path, fn)
			continue
		}
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			t.Fatalf("rel %s: %v", path, err)
		}
		fn(filepath.ToSlash(rel), string(content))
	}
}

// repoRoot returns the directory holding examples/ and catalog/.
func repoRoot(t *testing.T) string {
	t.Helper()
	for _, probe := range []string{
		"../../..", // go test from internal/emit/hcl
		"../../../..",
		".", "..", "../..",
	} {
		abs, err := filepath.Abs(probe)
		if err != nil {
			continue
		}
		if _, err := os.Stat(filepath.Join(abs, "examples", "vnet.yaml")); err == nil {
			if _, err := os.Stat(filepath.Join(abs, "catalog", "vnet.yaml")); err == nil {
				return abs
			}
		}
	}
	t.Fatal("could not locate the repo root (needs examples/vnet.yaml and catalog/vnet.yaml)")
	return ""
}

// mapsEqual reports whether two string maps are identical.
func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
