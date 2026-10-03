package catalog

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Load reads every .yaml entry under dir and returns the assembled catalog.
func Load(dir string) (*Catalog, error) {
	return LoadFS(os.DirFS(dir))
}

// LoadFS reads every .yaml entry reachable from fsys. Callers pass either a
// directory on disk or an embedded copy of the catalog data; the WASM build has
// no filesystem, so it uses the embedded one.
func LoadFS(fsys fs.FS) (*Catalog, error) {
	names, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("catalog: read dir: %w", err)
	}
	var yamlNames []string
	for _, e := range names {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".yaml") {
			yamlNames = append(yamlNames, e.Name())
		}
	}
	sort.Strings(yamlNames)

	c := &Catalog{Kinds: map[string]*Entry{}}
	for _, name := range yamlNames {
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, fmt.Errorf("catalog: read %s: %w", name, err)
		}
		var entry Entry
		if err := yaml.Unmarshal(data, &entry); err != nil {
			return nil, fmt.Errorf("catalog: parse %s: %w", name, err)
		}
		if entry.Kind == "" {
			return nil, fmt.Errorf("catalog: %s has no kind", name)
		}
		c.Kinds[entry.Kind] = &entry
	}
	return c, nil
}

// candidateDirs are the places the catalog data may live, relative to the
// working directory. The plan keeps catalog/ at the repo root as DATA.
var candidateDirs = []string{
	"catalog",
	"../catalog",
	"../../catalog",
	"../../../catalog",
}

// LoadDefault finds and loads the catalog data from the repo layout.
func LoadDefault() (*Catalog, error) {
	var lastErr error
	for _, dir := range candidateDirs {
		if _, err := os.Stat(dir); err != nil {
			lastErr = err
			continue
		}
		return Load(dir)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("catalog: no candidate directory found")
	}
	return nil, lastErr
}

// CleanPath joins dir and name the way LoadFS consumed them.
func CleanPath(dir, name string) string {
	return filepath.Join(dir, name)
}
