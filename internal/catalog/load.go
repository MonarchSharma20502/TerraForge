package catalog

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Load reads every .yaml entry under dir and returns the assembled catalog.
func Load(dir string) (*Catalog, error) {
	c := &Catalog{Kinds: map[string]*Entry{}}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("catalog: read dir %s: %w", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".yaml") {
			continue
		}
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("catalog: read %s: %w", path, err)
		}
		var entry Entry
		if err := yaml.Unmarshal(data, &entry); err != nil {
			return nil, fmt.Errorf("catalog: parse %s: %w", path, err)
		}
		if entry.Kind == "" {
			return nil, fmt.Errorf("catalog: %s has no kind", path)
		}
		c.Kinds[entry.Kind] = &entry
	}
	return c, nil
}

// LoadEmbedded reads catalog entries from the embedded filesystem. This is what
// the CLI and the WASM build use so the catalog ships inside the binary.
//
//go:embed *.yaml
var embedded embed.FS

// LoadDefault returns the catalog compiled into this binary.
func LoadDefault() (*Catalog, error) {
	c := &Catalog{Kinds: map[string]*Entry{}}
	entries, err := embedded.ReadDir(".")
	if err != nil {
		return nil, fmt.Errorf("catalog: embedded read: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		data, err := embedded.ReadFile(e.Name())
		if err != nil {
			return nil, fmt.Errorf("catalog: embedded %s: %w", e.Name(), err)
		}
		var entry Entry
		if err := yaml.Unmarshal(data, &entry); err != nil {
			return nil, fmt.Errorf("catalog: embedded parse %s: %w", e.Name(), err)
		}
		if entry.Kind == "" {
			return nil, fmt.Errorf("catalog: embedded %s has no kind", e.Name())
		}
		c.Kinds[entry.Kind] = &entry
	}
	return c, nil
}
