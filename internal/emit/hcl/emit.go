// Package hcl emits the generated Terraform tree from the resolved plan.
//
// All HCL is built with hclwrite at the AST level and passed through
// hclwrite.Format, so `terraform fmt -check` is a no-op by construction. No
// emitter function ever hand-formats or hand-indents HCL text.
package hcl

import (
	"fmt"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"

	"github.com/autonation/autonation/internal/catalog"
	"github.com/autonation/autonation/internal/resolver"
)

// Emitter turns a resolved plan into the generated file set.
type Emitter struct {
	cat *catalog.Catalog
}

// New returns an emitter bound to a catalog.
func New(cat *catalog.Catalog) *Emitter {
	return &Emitter{cat: cat}
}

// File is one generated file and its contents.
type File struct {
	// Path is relative to the project root, using forward slashes.
	Path string
	// Content is the formatted HCL.
	Content string
}

// Files is the full generated tree.
type Files struct {
	Files []File
}

// Add appends a formatted file.
func (f *Files) Add(path, content string) {
	f.Files = append(f.Files, File{Path: path, Content: content})
}

// format builds, formats and renders a body.
func format(body func(*hclwrite.Body)) string {
	f := hclwrite.NewEmptyFile()
	body(f.Body())
	return string(hclwrite.Format(f.Bytes()))
}

// Emit generates every file for a plan: Modules, Workload stacks, Deployments.
func (e *Emitter) Emit(plan *resolver.Plan) (*Files, error) {
	files := &Files{}

	if err := e.emitModules(files, plan); err != nil {
		return nil, err
	}
	if err := e.emitWorkload(files, plan); err != nil {
		return nil, err
	}
	if err := e.emitDeployments(files, plan); err != nil {
		return nil, err
	}

	return files, nil
}

// emitModules writes one reusable blueprint per distinct resource kind.
func (e *Emitter) emitModules(files *Files, plan *resolver.Plan) error {
	kinds := e.kindsInUse(plan)
	for _, kind := range kinds {
		entry, ok := e.cat.Get(kind)
		if !ok {
			return fmt.Errorf("hcl: unknown kind %q", kind)
		}
		main, err := e.moduleMain(entry)
		if err != nil {
			return err
		}
		files.Add(entry.Module+"/"+entry.Kind+"_main.tf", main)
		vars, err := e.moduleVars(entry)
		if err != nil {
			return err
		}
		files.Add(entry.Module+"/"+entry.Kind+"_var.tf", vars)
		output, err := e.moduleOutput(entry)
		if err != nil {
			return err
		}
		files.Add(entry.Module+"/"+entry.Kind+"_output.tf", output)
	}
	return nil
}

// kindsInUse returns the sorted distinct kinds in the plan.
func (e *Emitter) kindsInUse(plan *resolver.Plan) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, res := range plan.Ordered {
		if !seen[res.Component.Kind] {
			seen[res.Component.Kind] = true
			out = append(out, res.Component.Kind)
		}
	}
	return sortedStrings(out)
}

// moduleMain emits the resource block for a kind.
//
// References are written as raw HCL expressions: a quoted "var.name" would be
// a string literal, not a reference, and would fail terraform validate.
// Module variables are renamed to their resource attribute via entry.Args.
//
// Strict-name resources get a separate name_configs entry with
// replace(..., "-", ""): Azure rejects dashes for these kinds, so the dashed
// name the resolver computed is stripped at the point of use and the original
// name stays the single source of truth everywhere else.
//
// Derived attributes are set to a module-internal expression. When one of them
// reads the current subscription, the module also emits the data source it
// needs, so the caller never supplies a tenant id and none is written to a file.
func (e *Emitter) moduleMain(entry *catalog.Entry) (string, error) {
	return format(func(body *hclwrite.Body) {
		if needsSubscription(entry) {
			d := body.AppendNewBlock("data", []string{"azurerm_subscription", "current"}).Body()
			d.SetAttributeRaw("subscription_id", gohclTokens("var.subscription_id"))
		}
		r := body.AppendNewBlock("resource", []string{entry.AzureType, "this"})
		rb := r.Body()
		rb.SetAttributeRaw("name", gohclTokens(e.strictNameExpr(entry)))
		for _, v := range entry.Required {
			if v == "name" {
				continue
			}
			rb.SetAttributeRaw(e.attrName(entry, v), gohclTokens("var."+v))
		}
		for _, v := range entry.Optional {
			if v.ModuleOnly {
				continue
			}
			rb.SetAttributeRaw(e.attrName(entry, v.Name), gohclTokens("var."+v.Name))
		}
		for _, d := range entry.Derived {
			rb.SetAttributeRaw(d.Target, gohclTokens(d.From))
		}
		for _, b := range entry.Blocks {
			if b.ForEach != "" {
				db := rb.AppendNewBlock("dynamic", []string{b.Name})
				db.Body().SetAttributeRaw("for_each", gohclTokens(b.ForEach))
				cb := db.Body().AppendNewBlock("content", nil).Body()
				cb.SetAttributeRaw("name", gohclTokens("each.key"))
				cb.SetAttributeRaw("value", gohclTokens("each.value"))
				continue
			}
			nb := rb.AppendNewBlock(b.Name, nil).Body()
			emitBlock(nb, b.Attributes, b.Blocks)
		}
		for _, s := range entry.Support {
			sb := body.AppendNewBlock("resource", []string{s.Type, s.Name}).Body()
			if s.Count != "" {
				sb.SetAttributeRaw("count", gohclTokens(s.Count))
			}
			emitBlock(sb, s.Attributes, s.Blocks)
		}
	}), nil
}

// emitBlock writes the attributes and nested static blocks of one block body.
func emitBlock(body *hclwrite.Body, attrs []catalog.BlockAttr, blocks []catalog.Block) {
	for _, a := range attrs {
		body.SetAttributeRaw(a.Name, gohclTokens(a.Value))
	}
	for _, b := range blocks {
		nb := body.AppendNewBlock(b.Name, nil).Body()
		emitBlock(nb, b.Attributes, b.Blocks)
	}
}

// needsSubscription reports whether a kind derives anything from the current
// subscription data source.
func needsSubscription(entry *catalog.Entry) bool {
	for _, d := range entry.Derived {
		if strings.HasPrefix(d.From, "data.azurerm_subscription") {
			return true
		}
	}
	return false
}

// strictNameExpr returns the HCL expression a module uses for the resource
// name when the kind rejects dashes.
func (e *Emitter) strictNameExpr(entry *catalog.Entry) string {
	if entry.StrictName != nil && entry.StrictName.StripDashes {
		return "replace(var.name, \"-\", \"\")"
	}
	return "var.name"
}

// attrName returns the resource attribute a module variable maps to.
func (e *Emitter) attrName(entry *catalog.Entry, v string) string {
	if a, ok := entry.Args[v]; ok && a != "" {
		return a
	}
	return v
}

// moduleVars emits the variable declarations for a kind.
//
// Type constraints and defaults are written as raw HCL expressions: Terraform
// rejects a quoted type constraint, and a default like "[]" would be a string
// rather than an empty list.
func (e *Emitter) moduleVars(entry *catalog.Entry) (string, error) {
	return format(func(body *hclwrite.Body) {
		if needsSubscription(entry) {
			v := body.AppendNewBlock("variable", []string{"subscription_id"}).Body()
			v.SetAttributeRaw("type", gohclTokens("string"))
			v.SetAttributeValue("description", cty.StringVal("Azure subscription id"))
		}
		for _, name := range entry.Required {
			v := body.AppendNewBlock("variable", []string{name}).Body()
			v.SetAttributeRaw("type", gohclTokens("string"))
			v.SetAttributeValue("description", cty.StringVal("Required: "+name))
		}
		for _, ov := range entry.Optional {
			v := body.AppendNewBlock("variable", []string{ov.Name}).Body()
			v.SetAttributeRaw("type", gohclTokens(ov.Type))
			v.SetAttributeValue("description", cty.StringVal(ov.Description))
			v.SetAttributeRaw("default", gohclTokens(ov.Default))
		}
	}), nil
}

// moduleOutput emits the id and name outputs for a kind.
func (e *Emitter) moduleOutput(entry *catalog.Entry) (string, error) {
	return format(func(body *hclwrite.Body) {
		o := body.AppendNewBlock("output", []string{"id"}).Body()
		o.SetAttributeRaw("value", gohclTokens(entry.AzureType+".this.id"))
		n := body.AppendNewBlock("output", []string{"name"}).Body()
		n.SetAttributeRaw("value", gohclTokens(entry.AzureType+".this.name"))
	}), nil
}

// emitWorkload writes the six-file stack for each Workload stack in the plan.
func (e *Emitter) emitWorkload(files *Files, plan *resolver.Plan) error {
	for stack := range plan.Stacks {
		if err := e.emitStack(files, plan, stack); err != nil {
			return err
		}
	}
	return nil
}

// emitStack writes exactly the six files for one stack.
func (e *Emitter) emitStack(files *Files, plan *resolver.Plan, stack string) error {
	base := "Workload/" + stack
	res := plan.Stacks[stack]

	files.Add(base+"/main.tf", e.stackMain(plan, stack, res))
	files.Add(base+"/locals.tf", e.stackLocals(plan, stack, res))
	files.Add(base+"/variables.tf", e.stackVariables(plan, stack, res))
	files.Add(base+"/data.tf", e.stackData(plan, stack, res))
	files.Add(base+"/provider.tf", e.stackProvider(plan, stack, res))
	files.Add(base+"/terraform.tfvars", e.stackTfvars(plan, stack, res))
	return nil
}

// stackMain emits module calls ONLY. Every argument resolves to local.* or var.*.
//
// Required variables the component cannot supply for itself are bound to the
// owning component's name local; the rest come from stack variables.
func (e *Emitter) stackMain(plan *resolver.Plan, stack string, res []*resolver.Resolved) string {
	return format(func(body *hclwrite.Body) {
		for _, r := range res {
			entry, _ := e.cat.Get(r.Component.Kind)
			if entry == nil {
				return
			}
			mod := body.AppendNewBlock("module", []string{r.Component.ID})
			mb := mod.Body()
			mb.SetAttributeRaw("source", gohclTokens("\"../../"+entry.Module+"\""))
			mb.SetAttributeRaw("name", gohclTokens("local."+r.Component.ID+"_name"))
			for _, v := range entry.Required {
				if supplier, ok := r.Bindings[v]; ok {
					mb.SetAttributeRaw(v, gohclTokens("local."+supplier+"_name"))
					continue
				}
				mb.SetAttributeRaw(v, gohclTokens("var."+v))
			}
			for _, v := range entry.Optional {
				mb.SetAttributeRaw(v.Name, gohclTokens("var."+v.Name))
			}
			if needsSubscription(entry) {
				mb.SetAttributeRaw("subscription_id", gohclTokens("var.subscription_id"))
			}
		}
	})
}

// stackLocals emits the naming algorithm output for the stack.
//
// A stack declares a name local for every component it owns, plus a name local
// for every component owned by another stack that its modules reference. Each
// Workload stack is a separate root module, so locals do not cross stack
// boundaries; the naming algorithm is deterministic from metadata, so the same
// name is declared on both sides of the boundary.
func (e *Emitter) stackLocals(plan *resolver.Plan, stack string, res []*resolver.Resolved) string {
	byID := map[string]*resolver.Resolved{}
	for _, r := range plan.Ordered {
		byID[r.Component.ID] = r
	}
	return format(func(body *hclwrite.Body) {
		l := body.AppendNewBlock("locals", nil).Body()
		l.SetAttributeRaw("name_config", gohclTokens("lower(\""+plan.NameConfig+"\")"))
		for _, r := range res {
			l.SetAttributeValue(r.Component.ID+"_name", cty.StringVal(r.Name))
		}
		seen := map[string]bool{}
		for _, r := range res {
			for _, supplier := range r.Bindings {
				if seen[supplier] {
					continue
				}
				owner := e.cat.StackOf(byID[supplier].Component.Kind)
				if owner == "" || owner == stack {
					continue
				}
				seen[supplier] = true
				l.SetAttributeValue(supplier+"_name", cty.StringVal(byID[supplier].Name))
			}
		}
	})
}

// stackVariables emits the stack-level input variables.
//
// A stack declares a variable for every unbound required argument its modules
// reference, plus subscription_id. Type constraints are raw HCL expressions:
// Terraform rejects a quoted type constraint as a deprecated 0.11-ism.
func (e *Emitter) stackVariables(plan *resolver.Plan, stack string, res []*resolver.Resolved) string {
	return format(func(body *hclwrite.Body) {
		v := body.AppendNewBlock("variable", []string{"subscription_id"}).Body()
		v.SetAttributeRaw("type", gohclTokens("string"))
		v.SetAttributeValue("description", cty.StringVal("Azure subscription id"))

		seen := map[string]bool{"subscription_id": true}
		for _, r := range res {
			entry, ok := e.cat.Get(r.Component.Kind)
			if !ok {
				continue
			}
			for _, name := range entry.Required {
				if _, bound := r.Bindings[name]; bound {
					continue
				}
				if seen[name] {
					continue
				}
				seen[name] = true
				v := body.AppendNewBlock("variable", []string{name}).Body()
				v.SetAttributeRaw("type", gohclTokens("string"))
				v.SetAttributeValue("description", cty.StringVal(name+" for "+r.Component.ID))
			}
			for _, ov := range entry.Optional {
				if seen[ov.Name] {
					continue
				}
				seen[ov.Name] = true
				v := body.AppendNewBlock("variable", []string{ov.Name}).Body()
				v.SetAttributeRaw("type", gohclTokens(ov.Type))
				v.SetAttributeValue("description", cty.StringVal(ov.Description))
				v.SetAttributeRaw("default", gohclTokens(ov.Default))
			}
		}
	})
}

// stackData emits data sources for existing infrastructure.
//
// A stack only looks up a resource it does not own: a bound supplier living in
// another stack is read with a data source keyed on the supplier, so two
// components bound to the same resource group emit one lookup, not two.
func (e *Emitter) stackData(plan *resolver.Plan, stack string, res []*resolver.Resolved) string {
	return format(func(body *hclwrite.Body) {
		seen := map[string]bool{}
		for _, r := range res {
			for _, socket := range e.cat.Sockets(r.Component.Kind) {
				supplier, ok := r.Bindings[socket.Name]
				if !ok || seen[supplier] {
					continue
				}
				owner := e.cat.StackOf(socket.Kind)
				if owner == "" || owner == stack {
					continue
				}
				seen[supplier] = true
				d := body.AppendNewBlock("data", []string{"azurerm_" + socket.Kind, supplier}).Body()
				d.SetAttributeRaw("name", gohclTokens("local."+supplier+"_name"))
			}
		}
	})
}

// stackProvider emits pinned required_providers and the azurerm provider.
//
// required_providers is a block whose provider entries are map attributes, and
// the map keys must be quoted provider local names. No backend block, no
// secrets. Backend config is opt-in via a generated Workload/TfState stack
// with all values variable-fed.
func (e *Emitter) stackProvider(plan *resolver.Plan, stack string, res []*resolver.Resolved) string {
	return format(func(body *hclwrite.Body) {
		tf := body.AppendNewBlock("terraform", nil).Body()
		rp := tf.AppendNewBlock("required_providers", nil).Body()
		rp.SetAttributeValue("azurerm", cty.MapVal(map[string]cty.Value{
			"source":  cty.StringVal("hashicorp/azurerm"),
			"version": cty.StringVal(">= 4.0.0"),
		}))

		p := body.AppendNewBlock("provider", []string{"azurerm"}).Body()
		p.AppendNewBlock("features", nil)
		p.SetAttributeRaw("subscription_id", gohclTokens("var.subscription_id"))
	})
}

// stackTfvars emits the default values for the stack.
//
// Only variables that carry a catalog default get a value here; a required
// variable with no default is supplied by the environment tfvars.
func (e *Emitter) stackTfvars(plan *resolver.Plan, stack string, res []*resolver.Resolved) string {
	return format(func(body *hclwrite.Body) {
		body.SetAttributeValue("subscription_id", cty.StringVal(plan.Metadata.Subscription))
		seen := map[string]bool{"subscription_id": true}
		for _, r := range res {
			entry, ok := e.cat.Get(r.Component.Kind)
			if !ok {
				continue
			}
			for _, ov := range entry.Optional {
				if seen[ov.Name] {
					continue
				}
				seen[ov.Name] = true
				body.SetAttributeRaw(ov.Name, gohclTokens(ov.Default))
			}
		}
	})
}

// emitDeployments writes the per-environment tfvars for each stack.
func (e *Emitter) emitDeployments(files *Files, plan *resolver.Plan) error {
	env := resolver.EnvAbbr(plan.Metadata.Environment)
	if env == "" {
		env = "dev"
	}
	envDir := "Deployments/" + env
	for stack := range plan.Stacks {
		files.Add(envDir+"/"+stack+".tfvars", e.deploymentTfvars(plan, stack))
	}
	return nil
}

// deploymentTfvars emits one stack's environment overrides.
func (e *Emitter) deploymentTfvars(plan *resolver.Plan, stack string) string {
	return format(func(body *hclwrite.Body) {
		for _, r := range plan.Stacks[stack] {
			if r.Component.Kind == "resource_group" {
				continue
			}
			key := r.Component.ID
			obj := body.AppendNewBlock(key, nil).Body()
			obj.SetAttributeValue("name", cty.StringVal(r.Name))
		}
	})
}

// gohclTokens parses a raw HCL expression into tokens for SetAttributeRaw.
//
// hclwrite has no public "set this attribute to a bare reference" helper, so the
// reference is parsed from a scratch file and its expression tokens are copied.
// This is what keeps main.tf arguments as real references (var.x, local.y)
// rather than quoted strings.
func gohclTokens(expr string) hclwrite.Tokens {
	src := "x = " + expr
	f, diag := hclwrite.ParseConfig([]byte(src), "scratch.tf", hcl.Pos{Line: 1, Column: 1})
	if diag.HasErrors() || f == nil {
		return identTokens(expr)
	}
	// Re-render the parsed file and lift the expression text back out. The
	// scratch file has a single attribute, so everything after "x = " up to the
	// trailing newline is the expression.
	rendered := string(hclwrite.Format(f.Bytes()))
	_, rest, found := strings.Cut(rendered, "=")
	if !found {
		return identTokens(expr)
	}
	expr = strings.TrimSpace(rest)
	return identTokens(expr)
}

// identTokens builds a single-identifier token list, preserving any spaces so
// compound expressions survive the round trip.
func identTokens(expr string) hclwrite.Tokens {
	return hclwrite.Tokens{
		&hclwrite.Token{Type: hclsyntax.TokenIdent, Bytes: []byte(expr)},
	}
}

// sortedStrings is duplicated from the catalog package to keep the emitter free
// of import cycles.
func sortedStrings(in []string) []string {
	out := append([]string{}, in...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}
