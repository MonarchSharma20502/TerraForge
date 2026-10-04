// Package main is the WASM build of the Auto-nation core. The browser calls
// Generate with a spec document and gets back the full generated tree, the
// policy findings, and the diagrams - the same bytes the CLI writes.
//
// Build with
//
//	$GOOS=js $GOARCH=wasm go build -o web/public/autonation.wasm ./cmd/wasm
//
// The js build constraint keeps this package out of native builds: `go vet`
// and `go build ./...` on Windows would otherwise fail on syscall/js.
//
//go:build js

package main

import (
	"encoding/json"
	"syscall/js"

	catalogdata "github.com/autonation/autonation/catalog"
	"github.com/autonation/autonation/internal/catalog"
	"github.com/autonation/autonation/internal/emit/diagram"
	"github.com/autonation/autonation/internal/emit/hcl"
	"github.com/autonation/autonation/internal/policy"
	"github.com/autonation/autonation/internal/resolver"
	"github.com/autonation/autonation/internal/spec"
)

// GenerateResult is the payload returned to the browser.
type GenerateResult struct {
	// Files maps the generated path to its content, forward slashes.
	Files map[string]string `json:"files"`
	// Order is the deterministic file order for the tree view.
	Order []string `json:"order"`
	// Diagrams holds the Mermaid and D2 renderings.
	Diagrams Diagrams `json:"diagrams"`
	// Policy is the outcome of the five-rule gate.
	Policy PolicyResult `json:"policy"`
	// Error is empty when generation succeeded.
	Error string `json:"error,omitempty"`
}

// Diagrams holds the two renderings produced from one IR walk.
type Diagrams struct {
	Mermaid string `json:"mermaid"`
	D2      string `json:"d2"`
}

// PolicyResult is the gate outcome the findings panel renders.
type PolicyResult struct {
	Passed   bool          `json:"passed"`
	Findings []PolicyEntry `json:"findings"`
}

// PolicyEntry is one finding.
type PolicyEntry struct {
	Rule     string   `json:"rule"`
	Severity string   `json:"severity"`
	Message  string   `json:"message"`
	Files    []string `json:"files,omitempty"`
}

// CatalogEntry is the palette-facing projection of one resource kind.
type CatalogEntry struct {
	Kind      string   `json:"kind"`
	AzureType string   `json:"azureType"`
	Abbr      string   `json:"abbr"`
	Category  string   `json:"category"`
	Icon      string   `json:"icon"`
	Stack     string   `json:"stack"`
	Required  []string `json:"required"`
}

func main() {
	cat, err := catalogdata.Load()
	if err != nil {
		panic("autonation: load embedded catalog: " + err.Error())
	}

	js.Global().Set("autonationGenerate", js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) == 0 {
			return marshalResult(GenerateResult{Error: "missing spec document"})
		}
		return marshalResult(generate(cat, args[0].String()))
	}))

	js.Global().Set("autonationCatalog", js.FuncOf(func(this js.Value, args []js.Value) any {
		out := make([]CatalogEntry, 0, len(cat.KindList()))
		for _, kind := range cat.KindList() {
			e, ok := cat.Get(kind)
			if !ok {
				continue
			}
			out = append(out, CatalogEntry{
				Kind:      e.Kind,
				AzureType: e.AzureType,
				Abbr:      e.Abbr,
				Category:  e.Category,
				Icon:      e.Icon,
				Stack:     e.Stack,
				Required:  e.Required,
			})
		}
		b, _ := json.Marshal(out)
		return string(b)
	}))

	// Keep the module alive.
	select {}
}

// generate runs the whole pipeline and maps failures into the result rather than
// panicking, so a bad spec shows up in the findings panel instead of killing the
// preview.
func generate(cat *catalog.Catalog, document string) GenerateResult {
	bp, err := spec.ParseDocument([]byte(document))
	if err != nil {
		return GenerateResult{Error: "parse spec: " + err.Error()}
	}

	plan, err := resolver.New(cat).Resolve(bp)
	if err != nil {
		return GenerateResult{Error: "resolve: " + err.Error()}
	}

	files, err := hcl.New(cat).Emit(plan)
	if err != nil {
		return GenerateResult{Error: "emit hcl: " + err.Error()}
	}

	fileMap := map[string]string{}
	order := make([]string, 0, len(files.Files))
	for _, f := range files.Files {
		fileMap[f.Path] = f.Content
		order = append(order, f.Path)
	}

	diag := diagram.New(cat)
	fileMap["architecture.mmd"] = diag.Mermaid(plan)
	fileMap["architecture.d2"] = diag.D2(plan)
	order = append(order, "architecture.mmd", "architecture.d2")

	result := policy.New(cat).Check(plan, fileMap)

	out := GenerateResult{
		Files: fileMap,
		Order: order,
		Diagrams: Diagrams{
			Mermaid: fileMap["architecture.mmd"],
			D2:      fileMap["architecture.d2"],
		},
		Policy: PolicyResult{
			Passed:   result.Passed,
			Findings: make([]PolicyEntry, 0, len(result.Findings)),
		},
	}
	for _, f := range result.Findings {
		out.Policy.Findings = append(out.Policy.Findings, PolicyEntry{
			Rule:     string(f.Rule),
			Severity: string(f.Severity),
			Message:  f.Message,
			Files:    f.Files,
		})
	}
	return out
}

// marshalResult encodes the result for the JS bridge.
func marshalResult(r GenerateResult) string {
	b, err := json.Marshal(r)
	if err != nil {
		return `{"error":"encode result: ` + err.Error() + `"}`
	}
	return string(b)
}
