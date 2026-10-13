// Package analyze: the Analyzer orchestrates the pipeline. It owns no logic of
// its own - parse, classify, recommend, render - and is the single entry point
// both the CLI and the WASM bridge call.
package analyze

import (
	"sort"

	"github.com/autonation/autonation/internal/catalog"
)

// Analyzer turns Terraform source into an analysis result.
type Analyzer struct {
	classifier *Classifier
	rules      []Rule
}

// Rule is one best-practice recommendation. It receives the graph and the raw
// file contents and returns the findings it made.
type Rule interface {
	// ID is the rule identifier shown in the UI.
	ID() string
	// Check runs the rule against the analyzed input.
	Check(graph Graph, files map[string]string) []Finding
}

// New returns an analyzer bound to a catalog. The catalog is optional: a nil
// catalog still analyzes, with every node classified generically.
func New(cat *catalog.Catalog) *Analyzer {
	a := &Analyzer{
		classifier: NewClassifier(cat),
	}
	a.rules = defaultRules(a.classifier)
	return a
}

// Analyze runs the whole pipeline over the given files, keyed by path.
func (a *Analyzer) Analyze(files map[string]string) Result {
	graph, parseErrs := Parse(files)

	for i := range graph.Nodes {
		a.classifier.Classify(&graph.Nodes[i])
	}

	// Edge semantics are derived after classification because they depend on
	// the target's catalog kind.
	nodeByAddr := map[string]*Node{}
	for i := range graph.Nodes {
		nodeByAddr[graph.Nodes[i].ID] = &graph.Nodes[i]
	}
	for i := range graph.Edges {
		to := nodeByAddr[graph.Edges[i].To]
		toKind := ""
		if to != nil {
			toKind = to.Kind
		}
		from := nodeByAddr[graph.Edges[i].From]
		fromType := ""
		if from != nil {
			fromType = from.Type
		}
		graph.Edges[i].Semantic = SemanticFor(fromType, graph.Edges[i].Label, toKind)
	}

	var findings []Finding
	for _, rule := range a.rules {
		findings = append(findings, rule.Check(graph, files)...)
	}
	sortFindings(findings)

	return Result{
		Summary:  summarize(graph),
		Graph:    graph,
		Diagrams: Diagrams{Mermaid: Mermaid(graph), D2: D2(graph)},
		Findings: findings,
		Errors:   parseErrs,
	}
}

// sortFindings orders findings worst-first and then by rule id, so the
// recommendations panel leads with what matters.
func sortFindings(findings []Finding) {
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Severity != findings[j].Severity {
			return findings[i].Severity == Critical
		}
		return findings[i].Rule < findings[j].Rule
	})
}

// summarize counts what the code deploys.
func summarize(graph Graph) Summary {
	s := Summary{
		ByCategory: map[string]int{},
		ByProvider: map[string]int{},
	}
	providers := map[string]bool{}
	for _, n := range graph.Nodes {
		switch n.BlockKind {
		case BlockResource:
			s.ResourceCount++
			s.ByCategory[n.Category]++
			s.ByProvider[providerOf(n.Type)]++
		case BlockData:
			s.DataCount++
		case BlockModule:
			s.ModuleCallCount++
		}
		if n.Category != "" {
			providers[n.Category] = true
		}
	}
	for p := range providers {
		s.Providers = append(s.Providers, p)
	}
	sort.Strings(s.Providers)
	return s
}
