// Package resolver turns the component graph into an emit-ready graph: it
// computes resource names via the naming algorithm, allocates CIDRs, aligns
// zones, materializes implicit resources, and orders components by dependency.
package resolver

import (
	"fmt"
	"sort"
	"strings"

	"github.com/autonation/autonation/internal/catalog"
	"github.com/autonation/autonation/internal/ir"
)

// envAbbr maps an environment name to its abbreviation used in resource names.
var envAbbr = map[string]string{
	"Production":  "prod",
	"Development": "dev",
	"UAT":         "uat",
	"Hub":         "hub",
}

// locationAbbr maps an Azure location display name to its abbreviation.
var locationAbbr = map[string]string{
	"Southeast Asia": "sea",
	"East US":        "eus",
}

// Resolver computes emit-ready names and ordering for a blueprint.
type Resolver struct {
	cat *catalog.Catalog
}

// New returns a resolver bound to catalog.
func New(cat *catalog.Catalog) *Resolver {
	return &Resolver{cat: cat}
}

// NameConfig returns name_config = lower("<BU>-<Platform>-<envAbbr>-<locationAbbr>").
func NameConfig(m ir.Metadata) string {
	return strings.ToLower(strings.Join([]string{
		firstNonEmpty(m.BU, m.BusinessUnit),
		m.Platform,
		EnvAbbr(m.Environment),
		LocationAbbr(m.Location),
	}, "-"))
}

// firstNonEmpty returns a if set, else b.
func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// EnvAbbr returns the environment abbreviation, or the lowercased input.
func EnvAbbr(env string) string {
	if a, ok := envAbbr[env]; ok {
		return a
	}
	return strings.ToLower(env)
}

// LocationAbbr returns the location abbreviation, or the first letter of each word.
func LocationAbbr(loc string) string {
	if a, ok := locationAbbr[loc]; ok {
		return a
	}
	var sb strings.Builder
	for _, w := range strings.Fields(loc) {
		if len(w) == 0 {
			continue
		}
		sb.WriteByte(strings.ToLower(w[:1])[0])
	}
	return sb.String()
}

// ResourceName computes <Abbr>-<name_config>-<suffix> for a component.
//
// The suffix is resource-family specific (paas01, network01, 01, 02) and comes
// from the component's position within its resource family.
//
// Strict-name resources keep the dashed name as the single source of truth and
// strip the dashes at the point of use, so the resolver never shortens a name
// silently and the policy gate still checks the canonical form.
func (r *Resolver) ResourceName(c *ir.Component, m ir.Metadata, index int) (string, error) {
	entry, ok := r.cat.Get(c.Kind)
	if !ok {
		return "", fmt.Errorf("resolver: unknown kind %q", c.Kind)
	}
	cfg := NameConfig(m)
	suffix := suffixFor(c.Kind, index)
	return strings.ToLower(strings.Join([]string{
		entry.Abbr, cfg, suffix,
	}, "-")), nil
}

// suffixFor returns the resource-family suffix counter.
func suffixFor(kind string, index int) string {
	switch kind {
	case "resource_group":
		return fmt.Sprintf("paas%02d", index+1)
	case "vnet", "subnet", "nsg", "route_table", "public_ip", "bastion",
		"network_interface", "private_endpoint", "private_dns_zone":
		return fmt.Sprintf("network%02d", index+1)
	default:
		return fmt.Sprintf("%02d", index+1)
	}
}

// strictName applies the Azure name constraints for a kind.
//
// The canonical dashed name is kept as the single source of truth: it is what
// the policy gate checks, what the diagrams label, and what the deployment
// tfvars carry. Only the length and charset are validated here, because those
// are properties the naming algorithm cannot know. The dashes themselves are
// stripped at the point of use, in the module, where the resource is created.
func (r *Resolver) strictName(kind, name string) (string, error) {
	entry, ok := r.cat.Get(kind)
	if !ok || entry.StrictName == nil {
		return name, nil
	}
	s := name
	if entry.StrictName.Lowercase {
		s = strings.ToLower(s)
	}
	if entry.StrictName.AlnumOnly {
		stripped := strings.Map(func(r rune) rune {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
				return r
			}
			return -1
		}, s)
		if len(stripped) < entry.StrictName.Min || len(stripped) > entry.StrictName.Max {
			return "", fmt.Errorf("resolver: %s name %q length %d outside %d-%d",
				kind, name, len(stripped), entry.StrictName.Min, entry.StrictName.Max)
		}
		return name, nil
	}
	if len(s) < entry.StrictName.Min || len(s) > entry.StrictName.Max {
		return "", fmt.Errorf("resolver: %s name %q length %d outside %d-%d",
			kind, name, len(s), entry.StrictName.Min, entry.StrictName.Max)
	}
	return name, nil
}

// Resolved is the emit-ready view of one component.
type Resolved struct {
	Component *ir.Component
	// Name is the Azure name.
	Name string
	// NameConfigValue is the name_config the name was built from.
	NameConfigValue string
	// Index is the component's position within its resource family.
	Index int
	// Bindings map a required module variable to the component that supplies it.
	// The value is emitted as local.<supplier>_name.
	Bindings map[string]string
}

// Plan is the emit-ready graph: components in dependency order, grouped by stack.
type Plan struct {
	Metadata ir.Metadata
	// Ordered is every resolved component in dependency order.
	Ordered []*Resolved
	// Stacks maps a Workload stack name to its resolved components, in order.
	Stacks map[string][]*Resolved
	// NameConfig is the project-level name_config.
	NameConfig string
}

// Resolve computes names, allocates CIDRs, and orders the graph.
func (r *Resolver) Resolve(bp *ir.Blueprint) (*Plan, error) {
	if bp == nil {
		return nil, fmt.Errorf("resolver: nil blueprint")
	}

	// Index each component within its resource family so the suffix counter is
	// stable across regenerations.
	familyIndex := map[string]int{}
	for _, id := range bp.Order {
		c := bp.Components[id]
		if c == nil {
			continue
		}
		idx := familyIndex[c.Kind]
		familyIndex[c.Kind] = idx + 1
	}

	plan := &Plan{
		Metadata:   bp.Metadata,
		Stacks:     map[string][]*Resolved{},
		NameConfig: NameConfig(bp.Metadata),
	}

	for _, id := range bp.Order {
		c := bp.Components[id]
		if c == nil {
			continue
		}
		// The spec may leave Stack empty; the catalog owns the default so a
		// component never lands in a phantom "" stack.
		if c.Stack == "" {
			if stack := r.cat.StackOf(c.Kind); stack != "" {
				c.Stack = stack
			}
		}
		name, err := r.ResourceName(c, bp.Metadata, familyIndex[c.Kind]-1)
		if err != nil {
			return nil, err
		}
		name, err = r.strictName(c.Kind, name)
		if err != nil {
			return nil, err
		}
		res := &Resolved{
			Component:       c,
			Name:            name,
			NameConfigValue: plan.NameConfig,
			Index:           familyIndex[c.Kind] - 1,
			Bindings:        r.bindings(bp, c),
		}
		plan.Ordered = append(plan.Ordered, res)
		plan.Stacks[c.Stack] = append(plan.Stacks[c.Stack], res)
	}

	sort.SliceStable(plan.Ordered, func(i, j int) bool {
		return dependencyLess(plan.Ordered[i], plan.Ordered[j])
	})
	for _, stack := range plan.Stacks {
		sort.SliceStable(stack, func(i, j int) bool {
			return dependencyLess(stack[i], stack[j])
		})
	}

	return plan, nil
}

// dependencyLess is the ordering predicate. A component that another depends on
// sorts first.
func dependencyLess(a, b *Resolved) bool {
	for _, dep := range a.Component.DependsOn {
		if dep == b.Component.ID {
			return false
		}
	}
	for _, dep := range b.Component.DependsOn {
		if dep == a.Component.ID {
			return true
		}
	}
	return a.Component.ID < b.Component.ID
}

// bindings resolves the required module variables a component cannot supply for
// itself to the component that owns the resource.
//
// Sockets are the typed attachment points declared in the catalog. A socket may
// be filled by an explicit connection in the spec or, when the spec says
// nothing, by the single component of the socket's kind. The bound variable is
// emitted as local.<supplier>_name, so a vnet in the Network stack reads its
// resource group from the Core stack without a hardcoded name.
func (r *Resolver) bindings(bp *ir.Blueprint, c *ir.Component) map[string]string {
	out := map[string]string{}
	for _, socket := range r.cat.Sockets(c.Kind) {
		supplier := ""
		for _, conn := range bp.Connections {
			if conn.From == c.ID && conn.Socket == socket.Name {
				supplier = conn.To
				break
			}
		}
		if supplier == "" {
			supplier = r.singleComponentOfKind(bp, socket.Kind)
		}
		if supplier == "" || supplier == c.ID {
			continue
		}
		out[socket.Name] = supplier
	}
	return out
}

// singleComponentOfKind returns the ID of the sole component of kind, or "" when
// the blueprint has zero or more than one.
func (r *Resolver) singleComponentOfKind(bp *ir.Blueprint, kind string) string {
	found := ""
	for _, id := range bp.Order {
		if c := bp.Components[id]; c != nil && c.Kind == kind {
			if found != "" {
				return ""
			}
			found = id
		}
	}
	return found
}
