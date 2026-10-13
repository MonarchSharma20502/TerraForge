// Package main is the WASM build of the Auto-nation core. The browser calls
// Analyze with a map of Terraform files and gets back the architecture graph,
// the diagrams, and the recommendations - the same bytes the CLI writes.
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
	"github.com/autonation/autonation/internal/analyze"
	"github.com/autonation/autonation/internal/catalog"
)

// AnalyzeResult is the payload returned to the browser.
type AnalyzeResult struct {
	// Summary is the headline counts the header renders.
	Summary analyze.Summary `json:"summary"`
	// Graph is the nodes and edges the canvas renders.
	Graph analyze.Graph `json:"graph"`
	// Diagrams holds the Mermaid and D2 renderings.
	Diagrams analyze.Diagrams `json:"diagrams"`
	// Findings are the recommendations, worst first.
	Findings []analyze.Finding `json:"findings"`
	// Errors are the parse failures that limited the analysis.
	Errors []analyze.ParseError `json:"errors"`
	// Report is the Markdown rendering the report viewer shows.
	Report string `json:"report"`
	// Error is empty when analysis succeeded.
	Error string `json:"error,omitempty"`
}

// CatalogEntry is the palette-facing projection of one resource kind. The
// analyzer uses it to label nodes the catalog recognizes.
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

	// analyze runs the pipeline over a JSON map of path to file content, so the
	// browser can hand over a whole folder at once.
	js.Global().Set("autonationAnalyze", js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) == 0 {
			return marshalResult(AnalyzeResult{Error: "missing files document"})
		}
		return marshalResult(analyzeFiles(cat, args[0].String()))
	}))

	// catalog is still exposed so the UI can label nodes with icons and
	// categories without duplicating the catalog data in TypeScript.
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

// analyzeFiles decodes the file map, runs the pipeline, and maps failures into
// the result rather than panicking, so broken Terraform shows up as parse errors
// in the UI instead of killing the canvas.
func analyzeFiles(cat *catalog.Catalog, document string) AnalyzeResult {
	var files map[string]string
	if err := json.Unmarshal([]byte(document), &files); err != nil {
		return AnalyzeResult{Error: "decode files: " + err.Error()}
	}

	result := analyze.New(cat).Analyze(files)
	return AnalyzeResult{
		Summary:   result.Summary,
		Graph:     result.Graph,
		Diagrams:  result.Diagrams,
		Findings:  result.Findings,
		Errors:    result.Errors,
		Report:    analyze.Report(result),
	}
}

// marshalResult encodes the result for the JS bridge.
func marshalResult(r AnalyzeResult) string {
	b, err := json.Marshal(r)
	if err != nil {
		return `{"error":"encode result: ` + err.Error() + `"}`
	}
	return string(b)
}
