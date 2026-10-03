// Package diagram emits the architecture diagrams from one IR walk: Mermaid for
// the live UI and the generated .mmd file, D2 for the .d2 file and SVG/PNG export.
package diagram

import (
	"fmt"
	"strings"

	"github.com/autonation/autonation/internal/catalog"
	"github.com/autonation/autonation/internal/resolver"
)

// Emitter renders diagrams from a resolved plan.
type Emitter struct {
	cat *catalog.Catalog
}

// New returns a diagram emitter bound to a catalog.
func New(cat *catalog.Catalog) *Emitter {
	return &Emitter{cat: cat}
}

// Mermaid renders a flowchart of the plan.
//
// Nodes are components grouped by stack; edges are the dependency relationships.
// The same walk produces the live UI diagram and the generated .mmd file.
func (e *Emitter) Mermaid(plan *resolver.Plan) string {
	var sb strings.Builder
	sb.WriteString("flowchart TD\n")

	for _, res := range plan.Ordered {
		entry, ok := e.cat.Get(res.Component.Kind)
		label := res.Component.Kind
		if ok && entry.Icon != "" {
			label = entry.Icon
		}
		fmt.Fprintf(&sb, "  %s[\"%s\"]\n", nodeID(res.Component.ID), label)
	}

	for _, res := range plan.Ordered {
		for _, dep := range res.Component.DependsOn {
			fmt.Fprintf(&sb, "  %s --> %s\n", nodeID(dep), nodeID(res.Component.ID))
		}
	}

	return sb.String()
}

// D2 renders the same graph in D2 syntax for .d2 and SVG/PNG export.
func (e *Emitter) D2(plan *resolver.Plan) string {
	var sb strings.Builder
	for _, res := range plan.Ordered {
		fmt.Fprintf(&sb, "%s: %s\n", nodeID(res.Component.ID), res.Component.Kind)
	}
	for _, res := range plan.Ordered {
		for _, dep := range res.Component.DependsOn {
			fmt.Fprintf(&sb, "%s -> %s\n", nodeID(dep), nodeID(res.Component.ID))
		}
	}
	return sb.String()
}

// nodeID makes a string safe for use as a diagram node identifier.
func nodeID(id string) string {
	var sb strings.Builder
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			sb.WriteRune(r)
		default:
			sb.WriteRune('_')
		}
	}
	return sb.String()
}
