// Package catalog is the resource-kind catalog: one entry per Azure resource kind
// the generator knows how to emit. This is DATA, not code. Adding a resource kind
// is a catalog change plus a golden test; it is not an emitter code change unless
// the emitter genuinely cannot express it.
package catalog

// Entry describes one resource kind.
type Entry struct {
	// Kind is the catalog key, e.g. "resource_group", "vnet".
	Kind string `yaml:"kind"`

	// AzureType is the azurerm resource type, e.g. azurerm_resource_group.
	AzureType string `yaml:"azureType"`

	// Abbr is the naming abbreviation used by the naming algorithm:
	// <Abbr>-<BU>-<Platform>-<envAbbr>-<locationAbbr>-<nn>
	Abbr string `yaml:"abbr"`

	// Module is the path under Modules/ in the generated tree.
	Module string `yaml:"module"`

	// Stack is the default Workload stack for this kind.
	Stack string `yaml:"stack"`

	// Category groups the kind in the UI palette: Network, Compute, Data, Security.
	Category string `yaml:"category"`

	// Icon is the diagram glyph for this kind.
	Icon string `yaml:"icon"`

	// StrictName holds the Azure name constraints when the resource does not
	// accept the standard dashed name. Strict-name resources get a separate
	// name_configs entry with replace(..., "-", "").
	StrictName *StrictName `yaml:"strictName,omitempty"`

	// Required are the module variables with no default: name, rgName, location.
	Required []string `yaml:"required"`

	// Optional are the module variables that always carry a safe default.
	Optional []Var `yaml:"optional,omitempty"`

	// Args renames a module variable to the resource attribute it maps to when
	// the two differ, e.g. rgName -> resource_group_name. Keys are module
	// variable names, values are resource attribute names.
	Args map[string]string `yaml:"args,omitempty"`

	// Sockets are the typed attachment points this kind exposes.
	Sockets []Socket `yaml:"sockets,omitempty"`

	// Blocks are the nested blocks emitted as dynamic blocks.
	Blocks []Block `yaml:"blocks,omitempty"`
}

// StrictName holds Azure resource name constraints.
type StrictName struct {
	// Min and Max are the length bounds.
	Min int `yaml:"min"`
	// Max is the upper length bound.
	Max int `yaml:"max"`
	// Lowercase true means the name must be lowercase.
	Lowercase bool `yaml:"lowercase"`
	// AlnumOnly true means only alphanumerics are allowed (no dashes).
	AlnumOnly bool `yaml:"alnumOnly"`
	// StripDashes true means emit replace(name, "-", "").
	StripDashes bool `yaml:"stripDashes"`
}

// Var is one module variable.
type Var struct {
	Name string `yaml:"name"`
	// Type is the HCL type expression, e.g. string, list(string).
	Type string `yaml:"type"`
	// Default is the HCL default expression as emitted verbatim.
	Default string `yaml:"default"`
	// Description becomes the variable description.
	Description string `yaml:"description"`
}

// Socket is a typed attachment point exposed or consumed by a kind.
type Socket struct {
	// Name is the socket identifier, e.g. "subnet".
	Name string `yaml:"name"`
	// Kind is the resource kind that may fill it.
	Kind string `yaml:"kind"`
	// Required marks a socket that must be connected.
	Required bool `yaml:"required"`
}

// Block is a nested block emitted with dynamic.
type Block struct {
	// Name is the block type, e.g. "network_rules".
	Name string `yaml:"name"`
	// ForEach is the for_each expression over the variable.
	ForEach string `yaml:"forEach"`
}

// Catalog is the whole kind catalog keyed by Kind.
type Catalog struct {
	Kinds map[string]*Entry `yaml:"kinds"`
}

// Get returns the entry for kind and whether it exists.
func (c *Catalog) Get(kind string) (*Entry, bool) {
	e, ok := c.Kinds[kind]
	return e, ok
}

// KindList returns the sorted list of kind keys.
func (c *Catalog) KindList() []string {
	out := make([]string, 0, len(c.Kinds))
	for k := range c.Kinds {
		out = append(out, k)
	}
	sortStrings(out)
	return out
}

// Sockets returns the sockets kind exposes, or nil when it has none.
func (c *Catalog) Sockets(kind string) []Socket {
	if e, ok := c.Kinds[kind]; ok {
		return e.Sockets
	}
	return nil
}

// StackOf returns the default Workload stack for kind, or "" when unknown.
func (c *Catalog) StackOf(kind string) string {
	if e, ok := c.Kinds[kind]; ok {
		return e.Stack
	}
	return ""
}
