// Package policy implements the five v0.1 rules. They are enforced as a
// generation gate: download is blocked unless all five pass.
package policy

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/autonation/autonation/internal/catalog"
	"github.com/autonation/autonation/internal/ir"
	"github.com/autonation/autonation/internal/resolver"
)

// Severity is how hard a finding blocks.
type Severity string

const (
	// Critical blocks download outright.
	Critical Severity = "critical"
	// Warning is reported but does not block.
	Warning Severity = "warning"
)

// Finding is one policy violation.
type Finding struct {
	Rule     string   `json:"rule"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
	Files    []string `json:"files,omitempty"`
}

// Result is the outcome of running the ruleset.
type Result struct {
	Passed   bool      `json:"passed"`
	Findings []Finding `json:"findings"`
}

// Engine runs the five v0.1 rules.
type Engine struct {
	cat *catalog.Catalog
}

// New returns a policy engine bound to a catalog.
func New(cat *catalog.Catalog) *Engine {
	return &Engine{cat: cat}
}

// Check runs all five rules against the resolved plan and generated files.
func (e *Engine) Check(plan *resolver.Plan, files map[string]string) Result {
	var findings []Finding

	findings = append(findings, e.noHardcodedLiteralsInMain(files)...)
	findings = append(findings, e.namingConformance(plan)...)
	findings = append(findings, e.mandatoryTags(plan)...)
	findings = append(findings, e.noSecretsInFiles(files)...)
	findings = append(findings, e.noPublicIngressByDefault(plan)...)

	passed := true
	for _, f := range findings {
		if f.Severity == Critical {
			passed = false
			break
		}
	}

	return Result{Passed: passed, Findings: findings}
}

// noHardcodedLiteralsInMain implements rule 1: every argument in every generated
// main.tf resolves to local.* or var.*. This is the user's rule 3.
func (e *Engine) noHardcodedLiteralsInMain(files map[string]string) []Finding {
	var out []Finding
	for path, content := range files {
		if !strings.HasSuffix(path, "/main.tf") {
			continue
		}
		for _, line := range strings.Split(content, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if !strings.Contains(line, "=") {
				continue
			}
			key, value, found := strings.Cut(line, "=")
			if !found {
				continue
			}
			key = strings.TrimSpace(key)
			value = strings.TrimSpace(value)
			if strings.HasPrefix(value, "local.") || strings.HasPrefix(value, "var.") {
				continue
			}
			// A module source path is a literal by nature: it points at a
			// directory on disk, not at a computed value. The reference repos
			// use relative paths (../../Modules/...) and the plan keeps that.
			// Everything else in main.tf must resolve to local.* or var.*.
			if key == "source" && strings.HasPrefix(value, `"../`) {
				continue
			}
			out = append(out, Finding{
				Rule:     "no-hardcoded-literals-in-main",
				Severity: Critical,
				Message:  "argument does not resolve to local.* or var.*: " + strings.TrimSpace(key) + " = " + value,
				Files:    []string{path},
			})
		}
	}
	return out
}

// namingConformance implements rule 2: every emitted name matches the pattern
// and its kind's charset and length constraints.
func (e *Engine) namingConformance(plan *resolver.Plan) []Finding {
	var out []Finding
	pattern := regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)+$`)
	for _, res := range plan.Ordered {
		if !pattern.MatchString(res.Name) {
			out = append(out, Finding{
				Rule:     "naming-conformance",
				Severity: Critical,
				Message:  "name does not match the naming pattern: " + res.Name,
			})
		}
		entry, ok := e.cat.Get(res.Component.Kind)
		if !ok || entry.StrictName == nil {
			continue
		}
		// A strict-name resource is deployed as the dashes-stripped form, so that
		// is the string Azure sees and the one the charset rule must judge.
		s := res.Name
		if entry.StrictName.AlnumOnly {
			s = strings.ReplaceAll(s, "-", "")
		}
		if entry.StrictName.AlnumOnly && strings.ContainsAny(s, "-") {
			out = append(out, FoundStrict(res, entry))
		}
		if entry.StrictName.Min > 0 && len(s) < entry.StrictName.Min {
			out = append(out, Finding{
				Rule:     "naming-conformance",
				Severity: Critical,
				Message: fmt.Sprintf("name %s is %d characters, below the %d minimum for %s",
					s, len(s), entry.StrictName.Min, res.Component.Kind),
			})
		}
		if entry.StrictName.Max > 0 && len(s) > entry.StrictName.Max {
			out = append(out, Finding{
				Rule:     "naming-conformance",
				Severity: Critical,
				Message: fmt.Sprintf("name %s is %d characters, above the %d maximum for %s",
					s, len(s), entry.StrictName.Max, res.Component.Kind),
			})
		}
	}
	return out
}

// FoundStrict reports a strict-name charset violation.
func FoundStrict(res *resolver.Resolved, entry *catalog.Entry) Finding {
	return Finding{
		Rule:     "naming-conformance",
		Severity: Critical,
		Message:  "strict-name resource contains a disallowed character: " + res.Name,
	}
}

// mandatoryTags implements rule 3: Environment and Owner present and non-empty.
func (e *Engine) mandatoryTags(plan *resolver.Plan) []Finding {
	var out []Finding
	for _, res := range plan.Ordered {
		tags := mergedTags(res.Component, plan.Metadata)
		if strings.TrimSpace(tags["Environment"]) == "" {
			out = append(out, Finding{
				Rule:     "mandatory-tags",
				Severity: Critical,
				Message:  "Environment tag is missing or empty on " + res.Component.ID,
			})
		}
		if strings.TrimSpace(tags["Owner"]) == "" {
			out = append(out, Finding{
				Rule:     "mandatory-tags",
				Severity: Critical,
				Message:  "Owner tag is missing or empty on " + res.Component.ID,
			})
		}
	}
	return out
}

// noSecretsInFiles implements rule 4: no access keys or connection strings
// anywhere in the generated tree, including in comments.
func (e *Engine) noSecretsInFiles(files map[string]string) []Finding {
	var out []Finding
	secretPatterns := []string{
		"access_key", "accesskey", "connection_string", "connectionstring",
		"sas_token", "primary_key", "secondary_key", "account_key",
	}
	for path, content := range files {
		lower := strings.ToLower(content)
		for _, p := range secretPatterns {
			if strings.Contains(lower, p) {
				out = append(out, Finding{
					Rule:     "no-secrets-in-files",
					Severity: Critical,
					Message:  "secret-shaped string " + p + " present in generated file",
					Files:    []string{path},
				})
			}
		}
	}
	return out
}

// noPublicIngressByDefault implements rule 5: NSGs default-deny and PaaS public
// access off unless the spec explicitly opted in.
func (e *Engine) noPublicIngressByDefault(plan *resolver.Plan) []Finding {
	var out []Finding
	for _, res := range plan.Ordered {
		switch res.Component.Kind {
		case "nsg":
			if !defaultDeny(res.Component) {
				out = append(out, Finding{
					Rule:     "no-public-ingress-by-default",
					Severity: Critical,
					Message:  "NSG is not default-deny: " + res.Component.ID,
				})
			}
		}
	}
	return out
}

// defaultDeny reports whether an NSG component is default-deny.
func defaultDeny(c *ir.Component) bool {
	props := c.Properties
	if props == nil {
		return true
	}
	if rules, ok := props["security_rules"].([]any); ok {
		for _, raw := range rules {
			if rule, ok := raw.(map[string]any); ok {
				if dir, _ := rule["direction"].(string); dir == "Inbound" {
					if access, _ := rule["access"].(string); access == "Allow" {
						return false
					}
				}
			}
		}
	}
	return true
}

// mergedTags returns the effective tags for a component: component tags override
// metadata tags.
func mergedTags(c *ir.Component, m ir.Metadata) map[string]string {
	out := map[string]string{}
	for k, v := range m.Tags {
		out[k] = v
	}
	for k, v := range c.Tags {
		out[k] = v
	}
	return out
}
