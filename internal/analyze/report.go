// Package analyze: the report renderer. The CLI writes report.md from the
// analysis result; it is the human-readable counterpart to the JSON the WASM
// bridge returns.
package analyze

import (
	"fmt"
	"strings"
)

// Report renders a Markdown report of an analysis result.
func Report(r Result) string {
	var sb strings.Builder

	sb.WriteString("# Terraform analysis report\n\n")

	writeSummary(&sb, r.Summary)
	writeErrors(&sb, r.Errors)
	writeFindings(&sb, r.Findings)
	writeInventory(&sb, r.Graph)

	return sb.String()
}

// writeSummary renders the headline counts.
func writeSummary(sb *strings.Builder, s Summary) {
	sb.WriteString("## Summary\n\n")
	fmt.Fprintf(sb, "- %d resources, %d data sources, %d module calls\n",
		s.ResourceCount, s.DataCount, s.ModuleCallCount)
	if len(s.Providers) > 0 {
		fmt.Fprintf(sb, "- providers: %s\n", strings.Join(s.Providers, ", "))
	}
	if len(s.ByCategory) > 0 {
		var parts []string
		for _, k := range sortedStringKeys(s.ByCategory) {
			parts = append(parts, fmt.Sprintf("%s: %d", k, s.ByCategory[k]))
		}
		fmt.Fprintf(sb, "- by category: %s\n", strings.Join(parts, ", "))
	}
	sb.WriteString("\n")
}

// writeErrors renders the parse failures that limited the analysis.
func writeErrors(sb *strings.Builder, errs []ParseError) {
	if len(errs) == 0 {
		return
	}
	sb.WriteString("## Parse errors\n\n")
	for _, e := range errs {
		if e.Line > 0 {
			fmt.Fprintf(sb, "- **%s:%d** %s\n", e.File, e.Line, e.Message)
			continue
		}
		fmt.Fprintf(sb, "- **%s** %s\n", e.File, e.Message)
	}
	sb.WriteString("\n")
}

// writeFindings renders the recommendations, worst first.
func writeFindings(sb *strings.Builder, findings []Finding) {
	if len(findings) == 0 {
		sb.WriteString("## Recommendations\n\nNo findings. The code follows the recommended practices.\n\n")
		return
	}
	critical := 0
	for _, f := range findings {
		if f.Severity == Critical {
			critical++
		}
	}
	sb.WriteString("## Recommendations\n\n")
	fmt.Fprintf(sb, "%d finding(s): %d critical, %d warning\n\n",
		len(findings), critical, len(findings)-critical)
	for _, f := range findings {
		fmt.Fprintf(sb, "### %s (%s)\n\n%s\n\n%s\n\n",
			f.Rule, f.Severity, f.Message, f.Fix)
		if len(f.Files) > 0 {
			fmt.Fprintf(sb, "Files: %s\n\n", strings.Join(f.Files, ", "))
		}
	}
}

// writeInventory renders the resource inventory grouped by category, which is
// the "what deploys when you run this code" view.
func writeInventory(sb *strings.Builder, graph Graph) {
	if len(graph.Nodes) == 0 {
		return
	}
	sb.WriteString("## Inventory\n\n")
	byCategory := map[string][]Node{}
	for _, n := range graph.Nodes {
		if n.BlockKind != BlockResource {
			continue
		}
		cat := n.Category
		if cat == "" {
			cat = "Other"
		}
		byCategory[cat] = append(byCategory[cat], n)
	}
	for _, cat := range sortedStringKeys(byCategory) {
		fmt.Fprintf(sb, "### %s (%d)\n\n", cat, len(byCategory[cat]))
		for _, n := range byCategory[cat] {
			label := n.Name
			if label == "" {
				label = n.Type
			}
			fmt.Fprintf(sb, "- %s (%s)\n", label, n.Type)
		}
		sb.WriteString("\n")
	}
}
