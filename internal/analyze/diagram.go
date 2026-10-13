// Package analyze: the diagram renderers. One walk of the graph produces the
// Mermaid flowchart the UI shows and the D2 graph the .d2 file and SVG/PNG
// export use. Nodes are grouped by module so a module call reads as a boundary.
package analyze

import (
	"fmt"
	"sort"
	"strings"
)

// Mermaid renders a flowchart of the graph. Nodes are grouped by module call;
// edges are the references between blocks, styled by their semantic class.
func Mermaid(graph Graph) string {
	var sb strings.Builder
	sb.WriteString("flowchart TD\n")

	groups := groupNodes(graph.Nodes)
	for _, g := range groups {
		if g.name == "" {
			for _, n := range g.nodes {
				fmt.Fprintf(&sb, "  %s[\"%s\"]\n", mermaidID(n.ID), mermaidLabel(n))
			}
			continue
		}
		fmt.Fprintf(&sb, "  subgraph %s [\"%s\"]\n", mermaidID("mod:"+g.name), g.name)
		for _, n := range g.nodes {
			fmt.Fprintf(&sb, "    %s[\"%s\"]\n", mermaidID(n.ID), mermaidLabel(n))
		}
		sb.WriteString("  end\n")
	}

	for _, e := range graph.Edges {
		style := mermaidEdgeStyle(e.Semantic)
		if style == "" {
			fmt.Fprintf(&sb, "  %s --> %s\n", mermaidID(e.To), mermaidID(e.From))
			continue
		}
		fmt.Fprintf(&sb, "  %s -.- %s\n", mermaidID(e.To), mermaidID(e.From))
	}
	return sb.String()
}

// D2 renders the same graph in D2 syntax for the .d2 file and SVG/PNG export.
func D2(graph Graph) string {
	var sb strings.Builder
	groups := groupNodes(graph.Nodes)
	for _, g := range groups {
		if g.name == "" {
			for _, n := range g.nodes {
				fmt.Fprintf(&sb, "%s: %s\n", d2ID(n.ID), n.Type)
			}
			continue
		}
		fmt.Fprintf(&sb, "%s: %s {\n", d2ID("mod:"+g.name), g.name)
		for _, n := range g.nodes {
			fmt.Fprintf(&sb, "  %s: %s\n", d2ID(n.ID), n.Type)
		}
		sb.WriteString("}\n")
	}
	for _, e := range graph.Edges {
		fmt.Fprintf(&sb, "%s -> %s\n", d2ID(e.To), d2ID(e.From))
	}
	return sb.String()
}

// nodeGroup is a module call and the nodes declared inside it. The zero group
// holds the root-level blocks.
type nodeGroup struct {
	name  string
	nodes []Node
}

// groupNodes collects nodes by their module path, root first. Nodes inside a
// module are grouped under the module call's local name so the diagram reads as
// a boundary per module.
func groupNodes(nodes []Node) []nodeGroup {
	order := []string{}
	byName := map[string]*nodeGroup{}
	for _, n := range nodes {
		key := n.Group
		if n.Module == "" {
			key = ""
		}
		g, ok := byName[key]
		if !ok {
			g = &nodeGroup{name: key}
			byName[key] = g
			order = append(order, key)
		}
		g.nodes = append(g.nodes, n)
	}
	out := make([]nodeGroup, 0, len(order))
	for _, key := range order {
		out = append(out, *byName[key])
	}
	return out
}

// mermaidID makes a string safe for use as a Mermaid node identifier.
func mermaidID(id string) string {
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

// mermaidLabel is the text a node shows: the block's local name over its type.
func mermaidLabel(n Node) string {
	if n.Name != "" {
		return n.Name + "\\n" + n.Type
	}
	return n.Type
}

// mermaidEdgeStyle maps a semantic class onto a Mermaid link style. The classes
// mirror the legend the canvas draws.
func mermaidEdgeStyle(s EdgeSemantic) string {
	switch s {
	case SemanticPeering, SemanticVNetIntegration:
		return "peering"
	case SemanticVNetLink:
		return "vnet_link"
	case SemanticOnPrem:
		return "onprem"
	case SemanticWebRequest:
		return "web_request"
	default:
		return ""
	}
}

// d2ID makes a string safe for use as a D2 node identifier.
func d2ID(id string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		default:
			return '_'
		}
	}, id)
}

// sortedStringKeys is the stable ordering the renderers use for deterministic
// output over a string-keyed map of any value type.
func sortedStringKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
