package policy

import (
	"os"
	"strings"
	"testing"

	"github.com/autonation/autonation/internal/catalog"
	"github.com/autonation/autonation/internal/ir"
	"github.com/autonation/autonation/internal/resolver"
)

// osStat is a small indirection so the test file does not import os for one call.
func osStat(dir string) (os.FileInfo, error) {
	return os.Stat(dir)
}

// testCatalog loads the real catalog data from the repo root.
func testCatalog(t *testing.T) *catalog.Catalog {
	t.Helper()
	for _, dir := range []string{"../../catalog", "../../../catalog", "catalog"} {
		if _, err := osStat(dir); err == nil {
			c, err := catalog.Load(dir)
			if err != nil {
				t.Fatalf("catalog.Load %s: %v", dir, err)
			}
			return c
		}
	}
	t.Fatal("catalog data not found")
	return nil
}

// testPlan resolves a minimal three-resource blueprint with the mandatory tags.
func testPlan(t *testing.T, cat *catalog.Catalog) *resolver.Plan {
	t.Helper()
	bp := ir.NewBlueprint()
	bp.Metadata = ir.Metadata{
		BusinessUnit: "test",
		Platform:     "platform",
		Environment:  "Production",
		Location:     "Southeast Asia",
		Tags:         map[string]string{"Environment": "Production", "Owner": "team"},
	}
	bp.AddComponent(&ir.Component{ID: "hub", Kind: "resource_group", Stack: "Core"})
	bp.AddComponent(&ir.Component{ID: "spokevnet", Kind: "vnet", Stack: "Network"})
	bp.AddComponent(&ir.Component{ID: "aml", Kind: "subnet", Stack: "Network"})

	plan, err := resolver.New(cat).Resolve(bp)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	return plan
}

// TestNoHardcodedLiteralsInMain checks rule 1: every main.tf argument resolves to
// local.* or var.*, with source the one allowed literal.
func TestNoHardcodedLiteralsInMain(t *testing.T) {
	cat := testCatalog(t)
	e := New(cat)

	files := map[string]string{
		"Workload/Network/main.tf": "module \"vnet\" {\n  source = \"../../Modules/Network/Vnet\"\n  name   = local.vnet_name\n  tags   = var.tags\n}\n",
		"Workload/Bad/main.tf":     "module \"vnet\" {\n  address_space = [\"10.0.0.0/16\"]\n}\n",
	}
	findings := e.noHardcodedLiteralsInMain(files)
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1", len(findings))
	}
	if findings[0].Rule != "no-hardcoded-literals-in-main" {
		t.Errorf("rule = %q, want no-hardcoded-literals-in-main", findings[0].Rule)
	}
	if !strings.Contains(findings[0].Message, "10.0.0.0/16") {
		t.Errorf("message %q does not name the literal", findings[0].Message)
	}
}

// TestNamingConformance checks rule 2: names match the house-style pattern.
func TestNamingConformance(t *testing.T) {
	cat := testCatalog(t)
	plan := testPlan(t, cat)

	findings := New(cat).namingConformance(plan)
	if len(findings) != 0 {
		t.Fatalf("got %d naming findings, want 0: %v", len(findings), findings)
	}
}

// TestMandatoryTags checks rule 3: Environment and Owner present and non-empty.
func TestMandatoryTags(t *testing.T) {
	cat := testCatalog(t)
	plan := testPlan(t, cat)

	if len(New(cat).mandatoryTags(plan)) != 0 {
		t.Fatal("mandatory tags should pass when Environment and Owner are set")
	}

	// Strip the tags and the rule must fire twice per component.
	for _, res := range plan.Ordered {
		res.Component.Tags = nil
	}
	plan.Metadata.Tags = nil
	findings := New(cat).mandatoryTags(plan)
	if len(findings) == 0 {
		t.Fatal("mandatory tags should fail when Environment and Owner are absent")
	}
}

// TestNoSecretsInFiles checks rule 4: no secret-shaped strings anywhere.
func TestNoSecretsInFiles(t *testing.T) {
	cat := testCatalog(t)
	files := map[string]string{
		"Workload/Core/provider.tf": "# backend \"azurerm\" {\n#   access_key = \"realkey\"\n# }\n",
	}
	findings := New(cat).noSecretsInFiles(files)
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1", len(findings))
	}
	if findings[0].Rule != "no-secrets-in-files" {
		t.Errorf("rule = %q, want no-secrets-in-files", findings[0].Rule)
	}
}

// TestNoPublicIngressByDefault checks rule 5: NSGs default-deny.
func TestNoPublicIngressByDefault(t *testing.T) {
	cat := testCatalog(t)
	plan := testPlan(t, cat)

	if len(New(cat).noPublicIngressByDefault(plan)) != 0 {
		t.Fatal("no public ingress should pass when no NSG allows inbound")
	}

	plan.Ordered = append(plan.Ordered, &resolver.Resolved{
		Component: &ir.Component{
			ID:    "nsg1",
			Kind:  "nsg",
			Stack: "Network",
			Properties: map[string]any{
				"security_rules": []any{
					map[string]any{"direction": "Inbound", "access": "Allow"},
				},
			},
		},
	})
	findings := New(cat).noPublicIngressByDefault(plan)
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1", len(findings))
	}
}

// TestCheckPassesOnCleanPlan is the generation gate: a clean plan passes all five.
func TestCheckPassesOnCleanPlan(t *testing.T) {
	cat := testCatalog(t)
	plan := testPlan(t, cat)
	files := map[string]string{
		"Workload/Network/main.tf": "module \"vnet\" {\n  source = \"../../Modules/Network/Vnet\"\n  name   = local.vnet_name\n}\n",
	}

	result := New(cat).Check(plan, files)
	if !result.Passed {
		t.Fatalf("policy gate failed on a clean plan: %v", result.Findings)
	}
}

// TestCheckBlocksOnSecret is the reference-repo violation: a hardcoded access key
// in a commented-out backend block must block download.
func TestCheckBlocksOnSecret(t *testing.T) {
	cat := testCatalog(t)
	plan := testPlan(t, cat)
	files := map[string]string{
		"Workload/Core/provider.tf": "provider \"azurerm\" {\n  features {}\n}\n",
		"Workload/Core/main.tf":     "module \"rg\" {\n  source = \"../../Modules/resourceGroup\"\n  name   = local.rg_name\n}\n",
		"Workload/Bad/provider.tf":  "# backend \"azurerm\" {\n#   access_key = \"a2hhcmRjb3JlZGtleQ==\"\n# }\n",
	}

	result := New(cat).Check(plan, files)
	if result.Passed {
		t.Fatal("policy gate should block on a hardcoded access key")
	}
	found := false
	for _, f := range result.Findings {
		if f.Rule == "no-secrets-in-files" {
			found = true
		}
	}
	if !found {
		t.Error("no-secrets-in-files did not fire on the hardcoded access key")
	}
}

// TestCheckBlocksOnHardcodedCapacity is the reference-repo violation: a hardcoded
// capacity_id literal in main.tf must block download.
func TestCheckBlocksOnHardcodedCapacity(t *testing.T) {
	cat := testCatalog(t)
	plan := testPlan(t, cat)
	files := map[string]string{
		"Workload/Data/main.tf": "module \"databricks\" {\n  source      = \"../../Modules/Databricks\"\n  capacity_id = \"/subscriptions/abc/resourceGroups/xyz/providers/Microsoft.Databricks/workspaces/w\"\n}\n",
	}

	result := New(cat).Check(plan, files)
	if result.Passed {
		t.Fatal("policy gate should block on a hardcoded capacity_id")
	}
}
