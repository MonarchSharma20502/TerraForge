package resolver

import (
	"testing"

	"github.com/autonation/autonation/internal/catalog"
	"github.com/autonation/autonation/internal/ir"
)

// TestNameConfig checks the naming algorithm core: name_config is the lowercased
// dash-joined BU, Platform, envAbbr and locationAbbr.
func TestNameConfig(t *testing.T) {
	got := NameConfig(ir.Metadata{
		BusinessUnit: "test",
		Platform:     "platform",
		Environment:  "Production",
		Location:     "Southeast Asia",
	})
	want := "test-platform-prod-sea"
	if got != want {
		t.Errorf("NameConfig = %q, want %q", got, want)
	}
}

// TestEnvAbbr checks the environment abbreviation map.
func TestEnvAbbr(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"Production", "prod"},
		{"Development", "dev"},
		{"UAT", "uat"},
		{"Hub", "hub"},
		{"Sandbox", "sandbox"},
	}
	for _, c := range cases {
		if got := EnvAbbr(c.in); got != c.want {
			t.Errorf("EnvAbbr(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestLocationAbbr checks the location abbreviation map and the fallback.
func TestLocationAbbr(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"Southeast Asia", "sea"},
		{"East US", "eus"},
		{"North Europe", "ne"},
	}
	for _, c := range cases {
		if got := LocationAbbr(c.in); got != c.want {
			t.Errorf("LocationAbbr(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestResourceName checks the full naming algorithm against the house style:
// <Abbr>-<name_config>-<suffix>.
func TestResourceName(t *testing.T) {
	cat := &catalog.Catalog{Kinds: map[string]*catalog.Entry{
		"resource_group": {Kind: "resource_group", Abbr: "rg"},
		"vnet":           {Kind: "vnet", Abbr: "vnet"},
		"subnet":         {Kind: "subnet", Abbr: "snet"},
	}}
	r := New(cat)
	m := ir.Metadata{
		BusinessUnit: "test",
		Platform:     "platform",
		Environment:  "Production",
		Location:     "Southeast Asia",
	}
	cases := []struct {
		kind  string
		index int
		want  string
	}{
		{"resource_group", 0, "rg-test-platform-prod-sea-paas01"},
		{"vnet", 0, "vnet-test-platform-prod-sea-network01"},
		{"subnet", 0, "snet-test-platform-prod-sea-network01"},
		{"subnet", 1, "snet-test-platform-prod-sea-network02"},
	}
	for _, c := range cases {
		got, err := r.ResourceName(&ir.Component{ID: "x", Kind: c.kind}, m, c.index)
		if err != nil {
			t.Fatalf("ResourceName(%s, %d): %v", c.kind, c.index, err)
		}
		if got != c.want {
			t.Errorf("ResourceName(%s, %d) = %q, want %q", c.kind, c.index, got, c.want)
		}
	}
}

// TestResolveOrdersByDependency checks a component that another depends on sorts
// first.
func TestResolveOrdersByDependency(t *testing.T) {
	cat := &catalog.Catalog{Kinds: map[string]*catalog.Entry{
		"resource_group": {Kind: "resource_group", Abbr: "rg"},
		"vnet":           {Kind: "vnet", Abbr: "vnet"},
	}}
	bp := ir.NewBlueprint()
	bp.Metadata = ir.Metadata{
		BusinessUnit: "test",
		Platform:     "platform",
		Environment:  "Production",
		Location:     "Southeast Asia",
	}
	bp.AddComponent(&ir.Component{ID: "spokevnet", Kind: "vnet", DependsOn: []string{"hub"}})
	bp.AddComponent(&ir.Component{ID: "hub", Kind: "resource_group"})

	plan, err := New(cat).Resolve(bp)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(plan.Ordered) != 2 {
		t.Fatalf("Resolve produced %d components, want 2", len(plan.Ordered))
	}
	if plan.Ordered[0].Component.ID != "hub" {
		t.Errorf("hub should sort first, got %q", plan.Ordered[0].Component.ID)
	}
}
