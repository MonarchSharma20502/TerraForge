// Package analyze: the parser turns Terraform source into the block graph.
//
// HCL is parsed with the same library Terraform itself uses, so every form the
// user can write is accepted: .tf, .tf.json, nested blocks, dynamic blocks and
// module calls. A file that fails to parse is recorded as a ParseError and the
// rest of the input is still analyzed.
package analyze

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

// Parse reads the given Terraform files and returns the block graph plus any
// parse failures. Paths in the input map are used verbatim as node File values,
// so the caller controls how files are labelled.
func Parse(files map[string]string) (Graph, []ParseError) {
	parser := hclparse.NewParser()
	var errs []ParseError

	// Deterministic order so the graph is stable across runs.
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	for _, path := range paths {
		content := files[path]
		var file *hcl.File
		var diags hcl.Diagnostics
		if strings.HasSuffix(path, ".json") || strings.HasSuffix(path, ".tf.json") {
			file, diags = parser.ParseJSON([]byte(content), path)
		} else {
			file, diags = parser.ParseHCL([]byte(content), path)
		}
		for _, d := range diags {
			if d.Severity != hcl.DiagError {
				continue
			}
			line := 0
			if d.Subject != nil {
				line = d.Subject.Start.Line
			}
			errs = append(errs, ParseError{
				File:    path,
				Line:    line,
				Message: d.Summary + ": " + d.Detail,
			})
		}
		if file == nil {
			continue
		}
		rememberSource(path, []byte(content))
	}

	// Root blocks first, then the contents of each module call, so a module's
	// blocks carry the module path in their node IDs.
	var graph Graph
	for _, path := range paths {
		file, ok := parser.Files()[path]
		if !ok {
			continue
		}
		blocks := blocksFromFile(file)
		graph.Nodes = append(graph.Nodes, rootNodes(blocks, path)...)
	}

	graph.Edges = edges(parser)
	return graph, errs
}

// blocksFromFile returns the top-level blocks of one parsed file.
func blocksFromFile(file *hcl.File) hcl.Blocks {
	if body, ok := file.Body.(*hclsyntax.Body); ok {
		out := make(hcl.Blocks, 0, len(body.Blocks))
		for i := range body.Blocks {
			out = append(out, body.Blocks[i].AsHCLBlock())
		}
		return out
	}
	// JSON syntax: the body is not hclsyntax, so ask for the full content and
	// take the blocks from it.
	content, diags := file.Body.Content(jsonSchema)
	if diags.HasErrors() {
		return nil
	}
	return content.Blocks
}

// jsonSchema accepts every top-level block, matching the permissive behaviour
// the native syntax path has.
var jsonSchema = &hcl.BodySchema{
	Blocks: []hcl.BlockHeaderSchema{
		{Type: "resource", LabelNames: []string{"type", "name"}},
		{Type: "data", LabelNames: []string{"type", "name"}},
		{Type: "module", LabelNames: []string{"name"}},
		{Type: "provider", LabelNames: []string{"name"}},
		{Type: "terraform", LabelNames: []string{}},
		{Type: "variable", LabelNames: []string{"name"}},
		{Type: "output", LabelNames: []string{"name"}},
		{Type: "locals", LabelNames: []string{}},
	},
}

// rootNodes lifts the top-level resource, data and module blocks of one file
// into graph nodes at the root module.
func rootNodes(blocks hcl.Blocks, file string) []Node {
	var out []Node
	for _, b := range blocks {
		switch b.Type {
		case "resource", "data", "module":
			out = append(out, nodeFromBlock(b, "", file))
		}
	}
	return out
}

// nodeFromBlock builds the node for one block. Attributes are captured as their
// source text: literal values are rendered, expressions keep the source so the
// rules can pattern-match on what the user wrote.
func nodeFromBlock(b *hcl.Block, module, file string) Node {
	n := Node{
		ID:         blockID(b, module),
		Type:       blockType(b),
		Name:       b.Labels[len(b.Labels)-1],
		BlockKind:  BlockKind(b.Type),
		Module:     module,
		Attributes: map[string]string{},
		Blocks:     map[string]string{},
		File:       file,
		Line:       b.DefRange.Start.Line,
	}
	if b.Type == "module" {
		n.Group = b.Labels[0]
	}
	for name, attr := range blockAttributes(b) {
		n.Attributes[name] = attr
	}
	for name, src := range nestedBlocks(b) {
		n.Blocks[name] = src
	}
	return n
}

// blockID is the unique address of a block: <module>.<type>.<name>.
func blockID(b *hcl.Block, module string) string {
	parts := []string{}
	if module != "" {
		parts = append(parts, module)
	}
	parts = append(parts, blockType(b), b.Labels[len(b.Labels)-1])
	return strings.Join(parts, ".")
}

// blockType is the type label of a block: azurerm_resource_group for a resource,
// the module source path for a module call.
func blockType(b *hcl.Block) string {
	if b.Type == "module" {
		return "module." + b.Labels[0]
	}
	return b.Labels[0]
}

// blockAttributes returns the top-level attribute name to source-text map of a
// block. Nested blocks are skipped: they are not attributes, and the rules that
// need them walk the raw file instead.
func blockAttributes(b *hcl.Block) map[string]string {
	out := map[string]string{}
	body, ok := b.Body.(*hclsyntax.Body)
	if !ok {
		return out
	}
	for name, attr := range body.Attributes {
		out[name] = string(attr.Expr.Range().SliceBytes(srcFor(attr.Expr.Range().Filename)))
	}
	return out
}

// srcFor caches the source bytes of each parsed file so expression source text
// can be recovered by range.
var srcCache = map[string][]byte{}

// rememberSource stores a file's bytes for later range slicing.
func rememberSource(path string, content []byte) {
	srcCache[path] = content
}

// srcFor returns the cached source bytes of a file, or nil when unknown.
func srcFor(filename string) []byte {
	return srcCache[filename]
}

// edges walks every attribute of every block and turns references to other
// blocks into graph edges.
func edges(parser *hclparse.Parser) []Edge {
	var out []Edge
	seen := map[string]bool{}
	for _, file := range parser.Files() {
		for _, b := range blocksFromFile(file) {
			if b.Type != "resource" && b.Type != "data" && b.Type != "module" {
				continue
			}
			from := blockID(b, "")
			refs := blockReferences(b)
			for label, targets := range refs {
				for _, target := range targets {
					key := from + "->" + target + "@" + label
					if seen[key] {
						continue
					}
					seen[key] = true
					out = append(out, Edge{
						From:     from,
						To:       target,
						Label:    label,
						Semantic: SemanticDefault,
					})
				}
			}
		}
	}
	return out
}

// blockReferences returns attribute name to the block addresses that attribute
// references. A reference to azurerm_resource_group.hub.id becomes the address
// azurerm_resource_group.hub; the trailing attribute is dropped because edges
// are between blocks, not between a block and one property of another.
func blockReferences(b *hcl.Block) map[string][]string {
	out := map[string][]string{}
	body, ok := b.Body.(*hclsyntax.Body)
	if !ok {
		return out
	}
	for name, attr := range body.Attributes {
		var traversals []hcl.Traversal
		switch expr := attr.Expr.(type) {
		case *hclsyntax.ScopeTraversalExpr:
			traversals = append(traversals, expr.Traversal)
		case *hclsyntax.TemplateExpr:
			for _, part := range expr.Parts {
				if walk, ok := part.(*hclsyntax.ScopeTraversalExpr); ok {
					traversals = append(traversals, walk.Traversal)
				}
			}
		}
		for _, t := range traversals {
			addr, ok := traversalToAddress(t)
			if !ok {
				continue
			}
			out[name] = append(out[name], addr)
		}
	}
	return out
}

// traversalToAddress converts a traversal root-relative to a block address. It
// only accepts references rooted at a block type: azurerm_x.y, module.x, var.x
// and local.x are recognized; var and local resolve to nothing because their
// targets are not blocks in this file set.
func traversalToAddress(t hcl.Traversal) (string, bool) {
	if len(t) < 2 {
		return "", false
	}
	root, ok := t[0].(hcl.TraverseRoot)
	if !ok {
		return "", false
	}
	name, ok := t[1].(hcl.TraverseAttr)
	if !ok {
		return "", false
	}
	switch root.Name {
	case "module":
		// module.x.y resolves to the module call node, not to a block inside it.
		return "module." + name.Name, true
	case "var", "local", "each", "count", "self", "terraform", "path", "data":
		// data is handled below: data.x.y is a block address.
		if root.Name == "data" {
			return "data." + name.Name, false
		}
		return "", false
	default:
		// A resource reference: azurerm_resource_group.hub[...].id.
		return root.Name + "." + name.Name, true
	}
}

// nestedBlocks returns the nested block types of a block mapped to their raw
// source text, so rules can inspect an NSG's security_rule or a VM's os_disk
// without re-parsing the file. Repeated blocks of the same type are concatenated.
func nestedBlocks(b *hcl.Block) map[string]string {
	out := map[string]string{}
	body, ok := b.Body.(*hclsyntax.Body)
	if !ok {
		return out
	}
	for i := range body.Blocks {
		nb := body.Blocks[i]
		src := string(nb.Range().SliceBytes(srcFor(nb.Range().Filename)))
		if existing, ok := out[nb.Type]; ok {
			out[nb.Type] = existing + "\n" + src
			continue
		}
		out[nb.Type] = src
	}
	return out
}

// literalValue renders an expression as its literal string when it is a static
// value, and returns false for anything computed. Used by rules that must only
// fire on values the user actually typed.
func literalValue(expr hclsyntax.Expression) (string, bool) {
	val, diags := expr.Value(nil)
	if diags.HasErrors() {
		return "", false
	}
	if !val.IsKnown() || val.IsNull() {
		return "", false
	}
	switch val.Type() {
	case cty.String:
		return val.AsString(), true
	case cty.Bool:
		return fmt.Sprintf("%t", val.True()), true
	case cty.Number:
		return val.AsBigFloat().String(), true
	default:
		return "", false
	}
}
