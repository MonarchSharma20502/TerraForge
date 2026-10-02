// Package ir is the typed component graph that every downstream stage consumes.
//
// The spec is parsed into a Blueprint, which is a graph of Components connected
// by Connections through typed Sockets. Emitters, resolvers, policies and the UI
// all consume this graph. Nothing threads spec values directly into an emitter.
package ir

// Blueprint is the root of the component graph.
type Blueprint struct {
	// Metadata is the project-level context: business unit, platform, environment,
	// location, subscription. It feeds the naming algorithm and the tfvars globals.
	Metadata Metadata

	// Components keyed by ID. Order of insertion is preserved in Order so emitters
	// can walk the graph deterministically.
	Components map[string]*Component

	// Order is the stable list of component IDs.
	Order []string

	// Connections are the graph edges.
	Connections []*Connection
}

// Metadata holds the project-level naming and deployment context.
type Metadata struct {
	BusinessUnit  string
	Platform      string
	Environment   string
	Location      string
	Subscription  string
	BU            string // alias of BusinessUnit, kept for naming symmetry
	Owner         string
	Tags          map[string]string
}

// Component is a node in the graph: one Azure resource instance.
type Component struct {
	// ID is the graph-local identifier, unique within the blueprint. It is NOT the
	// Azure resource name - the resolver computes that from Metadata + Kind.
	ID string

	// Kind is the resource kind key, e.g. "resource_group", "vnet". It indexes the
	// catalog, which is data.
	Kind string

	// Name is the optional human label from the spec. Used as a hint only; the
	// resolver still produces the Azure name from the naming algorithm.
	Name string

	// Properties are the kind-specific inputs. Shape is defined by the catalog
	// entry and validated against the JSON Schema.
	Properties map[string]any

	// Tags are merged with Metadata.Tags at emit time.
	Tags map[string]string

	// Sockets are the typed attachment points this component exposes.
	Sockets []Socket

	// Stack is the Workload stack this component belongs to, e.g. "Network".
	// Drives the three-tier layout: one stack = one root module = one state.
	Stack string

	// DependsOn records explicit ordering constraints, resolved to component IDs.
	DependsOn []string
}

// Socket is a typed attachment point on a component.
type Socket struct {
	// Name is the socket identifier, e.g. "subnet", "resource_group".
	Name string

	// Kind is the type of component that may attach here.
	Kind string

	// Required marks a socket that must be connected for the blueprint to be valid.
	Required bool
}

// Connection is a graph edge with a semantic class.
type Connection struct {
	// From is the component ID that owns the socket.
	From string

	// To is the component ID that fills it.
	To string

	// Socket is the socket name on From.
	Socket string
}

// NewBlueprint returns an initialized empty blueprint.
func NewBlueprint() *Blueprint {
	return &Blueprint{
		Components: map[string]*Component{},
	}
}

// AddComponent inserts a component, preserving insertion order.
func (b *Blueprint) AddComponent(c *Component) {
	if c == nil {
		return
	}
	if _, exists := b.Components[c.ID]; !exists {
		b.Order = append(b.Order, c.ID)
	}
	b.Components[c.ID] = c
}

// AddConnection appends an edge.
func (b *Blueprint) AddConnection(c *Connection) {
	if c == nil {
		return
	}
	b.Connections = append(b.Connections, c)
}

// ComponentByID returns the component and whether it exists.
func (b *Blueprint) ComponentByID(id string) (*Component, bool) {
	c, ok := b.Components[id]
	return c, ok
}

// StackIDs returns the distinct stack names in stable first-seen order.
func (b *Blueprint) StackIDs() []string {
	seen := map[string]bool{}
	out := []string{}
	for _, id := range b.Order {
		c := b.Components[id]
		if c == nil || seen[c.Stack] {
			continue
		}
		seen[c.Stack] = true
		out = append(out, c.Stack)
	}
	return out
}

// ConnectionsFrom returns edges whose From is id.
func (b *Blueprint) ConnectionsFrom(id string) []*Connection {
	out := []*Connection{}
	for _, c := range b.Connections {
		if c.From == id {
			out = append(out, c)
		}
	}
	return out
}

// ConnectionsTo returns edges whose To is id.
func (b *Blueprint) ConnectionsTo(id string) []*Connection {
	out := []*Connection{}
	for _, c := range b.Connections {
		if c.To == id {
			out = append(out, c)
		}
	}
	return out
}
