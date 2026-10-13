// Package analyze turns Terraform source into a dependency graph plus a set of
// best-practice recommendations. It is the reverse of a generator: it reads
// whatever HCL it is given and reports what that code will deploy.
//
// The pipeline is parse -> graph -> classify -> recommend -> render. Every stage
// consumes the graph produced by the stage before it, and a syntax error in one
// file becomes a finding rather than a failed run.
package analyze

// Result is the outcome of analyzing one set of Terraform files. The CLI and the
// WASM bridge both serialize this same shape.
type Result struct {
	// Summary is the headline counts the header and the footer render.
	Summary Summary `json:"summary"`
	// Graph is the nodes and edges the canvas and the diagram emitters render.
	Graph Graph `json:"graph"`
	// Diagrams holds the Mermaid and D2 renderings of the graph.
	Diagrams Diagrams `json:"diagrams"`
	// Findings are the best-practice recommendations, worst first.
	Findings []Finding `json:"findings"`
	// Errors are the parse failures that prevented full analysis. They are
	// reported separately from findings so the UI can show them as broken input
	// rather than as a style violation.
	Errors []ParseError `json:"errors"`
}

// Summary is the headline view of what the code deploys.
type Summary struct {
	// ResourceCount is the number of resource blocks, excluding data sources.
	ResourceCount int `json:"resourceCount"`
	// DataCount is the number of data source blocks.
	DataCount int `json:"dataCount"`
	// ModuleCallCount is the number of module blocks, not the number of
	// resources inside them.
	ModuleCallCount int `json:"moduleCallCount"`
	// Providers are the distinct provider local names in use, sorted.
	Providers []string `json:"providers"`
	// ByCategory counts resources per catalog category for azurerm kinds.
	ByCategory map[string]int `json:"byCategory"`
	// ByProvider counts resources per provider local name.
	ByProvider map[string]int `json:"byProvider"`
}

// Graph is the analyzed topology.
type Graph struct {
	// Nodes are the resource, data and module blocks, in source order.
	Nodes []Node `json:"nodes"`
	// Edges are the references between blocks.
	Edges []Edge `json:"edges"`
}

// Node is one block in the graph.
type Node struct {
	// ID is the globally unique block address: <module path>.<type>.<name>,
	// with the module path empty at the root. It is the edge endpoint.
	ID string `json:"id"`
	// Type is the block type as written, e.g. azurerm_resource_group.
	Type string `json:"type"`
	// Kind is the catalog kind the type maps to, or "" when the provider is not
	// azurerm or the type is unknown to the catalog.
	Kind string `json:"kind"`
	// Name is the block's local name, e.g. hub.
	Name string `json:"name"`
	// BlockKind is resource, data or module.
	BlockKind BlockKind `json:"blockKind"`
	// Module is the module path the block lives in, "" at the root. Nested
	// modules are dot separated: root.module_a.module_b.
	Module string `json:"module"`
	// Group is the module call's local name when this node is inside a module,
	// so the canvas can draw one container per module call.
	Group string `json:"group"`
	// Icon is the catalog icon for an azurerm kind, else the kind text.
	Icon string `json:"icon"`
	// Category is the catalog category for an azurerm kind, else the provider
	// local name, so unknown providers still get a tier colour.
	Category string `json:"category"`
	// Attributes are the block's top-level attributes mapped to their source
	// text. Only statically known values are resolved; expressions are kept as
	// their source text so the rules can pattern-match them.
	Attributes map[string]string `json:"attributes"`
	// Blocks are the nested block types the block declares, mapped to the raw
	// source text of each. Rules that inspect nested blocks (an NSG's
	// security_rule, a VM's os_disk) read this instead of an attribute.
	Blocks map[string]string `json:"blocks"`
	// File is the file the block was declared in, relative to the input root.
	File string `json:"file"`
	// Line is the 1-based line of the block header.
	Line int `json:"line"`
}

// BlockKind is which HCL block type a node came from.
type BlockKind string

const (
	// BlockResource is a managed resource.
	BlockResource BlockKind = "resource"
	// BlockData is a data source.
	BlockData BlockKind = "data"
	// BlockModule is a module call.
	BlockModule BlockKind = "module"
)

// Edge is a reference from one block to another.
type Edge struct {
	// From is the node ID that carries the reference.
	From string `json:"from"`
	// To is the node ID that the reference resolves to.
	To string `json:"to"`
	// Label is the attribute or argument the reference passed through, e.g.
	// resource_group_name or vnet_id.
	Label string `json:"label"`
	// Semantic is the legend class the canvas draws this edge as.
	Semantic EdgeSemantic `json:"semantic"`
}

// EdgeSemantic is the legend class of an edge, mirroring the EDGE_STYLE map in
// the canvas so Go and the UI agree on connector styling.
type EdgeSemantic string

const (
	// SemanticDefault is a plain dependency arrow.
	SemanticDefault EdgeSemantic = "default"
	// SemanticPeering is a vnet-to-vnet peering.
	SemanticPeering EdgeSemantic = "peering"
	// SemanticVNetIntegration is a subnet or vnet reference from a resource.
	SemanticVNetIntegration EdgeSemantic = "vnet_integration"
	// SemanticVNetLink is a private DNS zone linked to a vnet.
	SemanticVNetLink EdgeSemantic = "vnet_link"
	// SemanticWebRequest is a compute resource reaching a service.
	SemanticWebRequest EdgeSemantic = "web_request"
	// SemanticOnPrem is a gateway connection to an on-premises network.
	SemanticOnPrem EdgeSemantic = "onprem"
)

// Diagrams holds the two renderings produced from one graph walk.
type Diagrams struct {
	Mermaid string `json:"mermaid"`
	D2      string `json:"d2"`
}

// Finding is one recommendation.
type Finding struct {
	// Rule is the rule id, e.g. no-hardcoded-secrets.
	Rule string `json:"rule"`
	// Severity is how bad this finding is. It colours the recommendation and
	// nothing else: there is no download gate in an analyzer.
	Severity Severity `json:"severity"`
	// Message is what the user did and why it matters.
	Message string `json:"message"`
	// Fix is the one-line remediation shown under the message.
	Fix string `json:"fix"`
	// Files are the files the finding is about.
	Files []string `json:"files,omitempty"`
}

// Severity is how hard a finding is.
type Severity string

const (
	// Critical is a finding that is exploitable or will fail at apply time.
	Critical Severity = "critical"
	// Warning is a finding that is a best-practice gap.
	Warning Severity = "warning"
)

// ParseError is one file that could not be parsed.
type ParseError struct {
	// File is the file that failed, relative to the input root.
	File string `json:"file"`
	// Line is the 1-based line of the failure, 0 when unknown.
	Line int `json:"line"`
	// Message is the parser diagnostic.
	Message string `json:"message"`
}
