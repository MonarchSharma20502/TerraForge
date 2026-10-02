// Package spec parses the Auto-nation DSL into the typed component graph.
//
// The DSL is JSON-Schema-backed and shaped to the house style: globals (location,
// subscription_id, name_config, tags) plus one nested object per resource family
// holding optional overrides - the same shape as the reference tfvars, so a
// generated Deployments/Dev/network.tfvars is valid input back into the generator.
package spec

import (
	"encoding/json"
	"fmt"

	"github.com/autonation/autonation/internal/ir"
)

// Spec is the top-level DSL document.
type Spec struct {
	// APIVersion lets the parser reject an unsupported document.
	APIVersion string `json:"apiVersion" yaml:"apiVersion"`

	// Metadata is the project-level naming and deployment context.
	Metadata Metadata `json:"metadata" yaml:"metadata"`

	// Resources is a nested object per resource family, mirroring the reference
	// tfvars shape: network = { spokevnet = { ... } }.
	Resources map[string]map[string]any `json:"resources" yaml:"resources"`
}

// Metadata is the project-level context that feeds the naming algorithm.
type Metadata struct {
	BusinessUnit string            `json:"businessUnit" yaml:"businessUnit"`
	Platform     string            `json:"platform" yaml:"platform"`
	Environment  string            `json:"environment" yaml:"environment"`
	Location     string            `json:"location" yaml:"location"`
	Subscription string            `json:"subscriptionId" yaml:"subscriptionId"`
	Owner        string            `json:"owner" yaml:"owner"`
	Tags         map[string]string `json:"tags" yaml:"tags"`
}

// Parse converts a decoded Spec into the typed component graph.
//
// Each resource family entry becomes one Component. The family key is the
// resource Kind (a catalog key); the inner key is the component ID.
func Parse(s *Spec) (*ir.Blueprint, error) {
	if s == nil {
		return nil, fmt.Errorf("spec: nil document")
	}
	if s.APIVersion != "" && s.APIVersion != "autonation/v1" {
		return nil, fmt.Errorf("spec: unsupported apiVersion %q, want %q", s.APIVersion, "autonation/v1")
	}

	bp := ir.NewBlueprint()
	bp.Metadata = ir.Metadata{
		BusinessUnit: s.Metadata.BusinessUnit,
		Platform:     s.Metadata.Platform,
		Environment:  s.Metadata.Environment,
		Location:     s.Metadata.Location,
		Subscription: s.Metadata.Subscription,
		Owner:        s.Metadata.Owner,
		Tags:         s.Metadata.Tags,
		BU:           s.Metadata.BusinessUnit,
	}

	for family, items := range s.Resources {
		if items == nil {
			continue
		}
		for id, props := range items {
			c, err := buildComponent(family, id, props)
			if err != nil {
				return nil, fmt.Errorf("spec: %s/%s: %w", family, id, err)
			}
			bp.AddComponent(c)
		}
	}

	return bp, nil
}

// ParseJSON parses a JSON spec document.
func ParseJSON(data []byte) (*ir.Blueprint, error) {
	var s Spec
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("spec: invalid json: %w", err)
	}
	return Parse(&s)
}

// buildComponent lifts one resource family entry into a component.
func buildComponent(kind, id string, props any) (*ir.Component, error) {
	c := &ir.Component{
		ID:       id,
		Kind:     kind,
		Stack:    stackFor(kind),
		Properties: map[string]any{},
	}
	if id == "" {
		return nil, fmt.Errorf("empty component id")
	}
	if kind == "" {
		return nil, fmt.Errorf("empty resource kind")
	}

	switch v := props.(type) {
	case map[string]any:
		for key, val := range v {
			if key == "tags" {
				if tags, ok := val.(map[string]any); ok {
					c.Tags = map[string]string{}
					for tk, tv := range tags {
						c.Tags[tk] = fmt.Sprint(tv)
					}
				}
				continue
			}
			if key == "stack" {
				if stack, ok := val.(string); ok {
					c.Stack = stack
				}
				continue
			}
			c.Properties[key] = val
		}
	case nil:
		// A resource with no overrides is valid; the catalog defaults apply.
	default:
		return nil, fmt.Errorf("expected object, got %T", props)
	}

	return c, nil
}

// stackFor maps a resource kind to its Workload stack.
//
// One Workload stack = one Terraform root module = one state. The mapping is
// deliberately coarse: the resolver may reassign a component when a connection
// pulls it into another stack's composition.
func stackFor(kind string) string {
	switch kind {
	case "resource_group":
		return "Core"
	case "vnet", "subnet", "nsg", "route_table", "public_ip", "bastion",
		"network_interface", "private_endpoint", "private_dns_zone":
		return "Network"
	case "windows_vm", "linux_vm":
		return "Compute"
	case "storage_account", "key_vault", "log_analytics":
		return "Data"
	default:
		return "Core"
	}
}
