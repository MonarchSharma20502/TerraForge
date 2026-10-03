// Package workspace writes the generated file tree to disk plus the
// .autonation.yaml manifest that makes regeneration a zero-diff no-op.
package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

// Manifest records what was generated and from what. It is what makes
// regeneration idempotent: same spec + same generator version => zero diff.
type Manifest struct {
	APIVersion       string            `yaml:"apiVersion"`
	GeneratorVersion string            `yaml:"generatorVersion"`
	SpecHash         string            `yaml:"specHash"`
	Files            map[string]string `yaml:"files"`
}

// Writer writes files into a project root.
type Writer struct {
	root string
	mu   sync.Mutex
}

// New returns a writer for root.
func New(root string) *Writer {
	return &Writer{root: root}
}

// Write writes the given files relative to root, then writes the manifest.
func (w *Writer) Write(files map[string]string, manifest *Manifest) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := os.MkdirAll(w.root, 0o755); err != nil {
		return fmt.Errorf("workspace: mkdir %s: %w", w.root, err)
	}

	for path, content := range files {
		full := filepath.Join(w.root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return fmt.Errorf("workspace: mkdir %s: %w", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			return fmt.Errorf("workspace: write %s: %w", full, err)
		}
	}

	return w.writeManifest(manifest)
}

// writeManifest writes .autonation.yaml at the project root.
func (w *Writer) writeManifest(m *Manifest) error {
	if m == nil {
		return nil
	}
	data, err := MarshalManifest(m)
	if err != nil {
		return err
	}
	full := filepath.Join(w.root, ".autonation.yaml")
	if err := os.WriteFile(full, data, 0o644); err != nil {
		return fmt.Errorf("workspace: write manifest %s: %w", full, err)
	}
	return nil
}

// HashFiles computes the per-file sha256 map for the manifest.
func HashFiles(files map[string]string) map[string]string {
	out := make(map[string]string, len(files))
	for path, content := range files {
		sum := sha256.Sum256([]byte(content))
		out[path] = hex.EncodeToString(sum[:])
	}
	return out
}

// HashSpec computes the sha256 of the raw spec document.
func HashSpec(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// MarshalManifest renders a manifest to the exact bytes written to
// .autonation.yaml, so tests can rebuild the file byte for byte.
func MarshalManifest(m *Manifest) ([]byte, error) {
	data, err := yaml.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("workspace: marshal manifest: %w", err)
	}
	return data, nil
}

// CleanPath joins root and rel safely.
func CleanPath(root, rel string) string {
	return filepath.Join(root, filepath.FromSlash(rel))
}
