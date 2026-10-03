// Package catalogdata embeds the catalog YAML so the WASM build can load the
// catalog without a filesystem. The embedded files ARE the catalog/ directory
// the CLI reads from disk - one source of truth, two read paths.
package catalogdata

import (
	"embed"
	"io/fs"

	"github.com/autonation/autonation/internal/catalog"
)

// embedded holds every .yaml file in the repo's catalog/ directory.
//
//go:embed *.yaml
var embedded embed.FS

// FS returns the catalog data as an fs.FS.
func FS() fs.FS {
	return embedded
}

// Load reads the embedded catalog entries into a catalog.Catalog.
func Load() (*catalog.Catalog, error) {
	return catalog.LoadFS(FS())
}
