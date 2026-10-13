// Package analyze: the operational-readiness rules. These find code that deploys
// but is missing the lifecycle controls the template expects: backup, diagnostic
// logging, resource locks, and pinned provider and module versions.

package analyze

import (
	"strconv"
	"strings"
)

// noBackup flags virtual machines with no recovery services vault or backup
// policy anywhere in the code. A VM with no backup is a data-loss incident
// waiting for the first disk failure.
type noBackup struct {
	c *Classifier
}

func (r noBackup) ID() string { return "no-backup" }

func (r noBackup) Check(graph Graph, files map[string]string) []Finding {
	var out []Finding
	hasBackup := false
	var vms []Node
	for _, n := range graph.Nodes {
		if n.BlockKind != BlockResource {
			continue
		}
		switch n.Kind {
		case "recovery_services_vault", "backup_policy_vm":
			hasBackup = true
		case "windows_vm", "linux_vm", "virtual_machine":
			vms = append(vms, n)
		}
	}
	if hasBackup || len(vms) == 0 {
		return out
	}
	names := make([]string, 0, len(vms))
	for _, v := range vms {
		names = append(names, v.Name)
	}
	out = append(out, finding(
		r.ID(), Warning,
		itoa(len(vms))+" virtual machine(s) ("+strings.Join(names, ", ")+") with no backup",
		"add an azurerm_recovery_services_vault and an azurerm_backup_policy_vm",
	))
	return out
}

// noDiagnosticLogging flags PaaS resources with no diagnostic setting anywhere.
// Without one, the service's logs go nowhere and an incident is un diagnosable.
type noDiagnosticLogging struct {
	c *Classifier
}

func (r noDiagnosticLogging) ID() string { return "no-diagnostic-logging" }

func (r noDiagnosticLogging) Check(graph Graph, files map[string]string) []Finding {
	var out []Finding
	hasDiagnostic := false
	var paas []Node
	for _, n := range graph.Nodes {
		if n.BlockKind != BlockResource {
			continue
		}
		switch n.Kind {
		case "monitor_diagnostic_setting":
			hasDiagnostic = true
		case "storage_account", "key_vault", "cosmosdb_account", "mssql_server",
			"postgresql_flexible_server", "eventhub_namespace", "servicebus_namespace",
			"application_insights", "log_analytics", "redis_cache", "databricks_workspace",
			"api_management", "kubernetes_cluster", "network_security_group":
			paas = append(paas, n)
		}
	}
	if hasDiagnostic || len(paas) == 0 {
		return out
	}
	names := make([]string, 0, len(paas))
	for _, p := range paas {
		names = append(names, p.Name)
	}
	out = append(out, finding(
		r.ID(), Warning,
		itoa(len(paas))+" PaaS resource(s) ("+strings.Join(names, ", ")+") with no diagnostic setting",
		"add an azurerm_monitor_diagnostic_setting targeting a log analytics workspace",
	))
	return out
}

// noResourceLocks flags the absence of any management lock. An accidental delete
// of a resource group is the classic production incident a lock prevents.
type noResourceLocks struct{}

func (r noResourceLocks) ID() string { return "no-resource-locks" }

func (r noResourceLocks) Check(graph Graph, files map[string]string) []Finding {
	var out []Finding
	hasLock := false
	resources := 0
	for _, n := range graph.Nodes {
		if n.BlockKind != BlockResource {
			continue
		}
		if n.Kind == "management_lock" {
			hasLock = true
		}
		if n.Kind == "resource_group" {
			resources++
		}
	}
	if hasLock || resources == 0 {
		return out
	}
	out = append(out, finding(
		r.ID(), Warning,
		itoa(resources)+" resource group(s) with no management lock",
		"add an azurerm_management_lock at the resource group level",
	))
	return out
}

// noVMWithoutAvailability flags a VM that is neither in an availability set nor
// in a zone nor part of a scale set. A single unzoned VM has no SLA.
type noVMWithoutAvailability struct{}

func (r noVMWithoutAvailability) ID() string { return "no-vm-without-availability" }

func (r noVMWithoutAvailability) Check(graph Graph, files map[string]string) []Finding {
	var out []Finding
	for _, n := range graph.Nodes {
		if n.BlockKind != BlockResource {
			continue
		}
		switch n.Kind {
		case "windows_vm", "linux_vm", "virtual_machine":
			if vmHasAvailability(n, graph) {
				continue
			}
			out = append(out, finding(
				r.ID(), Warning,
				n.Name+" is not in an availability set, an availability zone or a scale set",
				"place the VM in an availability set or specify availability zones",
				n.File,
			))
		}
	}
	return out
}

// vmHasAvailability reports whether a VM has an availability set reference, a
// zone list, or is itself part of a scale set.
func vmHasAvailability(n Node, graph Graph) bool {
	if v, ok := n.Attributes["availability_set_id"]; ok && v != "" && v != "null" {
		return true
	}
	if v, ok := n.Attributes["zones"]; ok && v != "" && v != "[]" && v != "null" {
		return true
	}
	for _, e := range graph.Edges {
		if e.From != n.ID {
			continue
		}
		if strings.Contains(strings.ToLower(e.Label), "availability_set") {
			return true
		}
	}
	return false
}

// unpinnedProviderVersion flags a provider block with no version constraint, or
// one that is open-ended. An unpinned provider is a silent breaking change on
// the next terraform init.
type unpinnedProviderVersion struct{}

func (r unpinnedProviderVersion) ID() string { return "unpinned-provider-version" }

func (r unpinnedProviderVersion) Check(graph Graph, files map[string]string) []Finding {
	var out []Finding
	for path, content := range files {
		if !strings.Contains(content, "required_providers") {
			continue
		}
		// A required_providers block pins versions per provider. Its absence in a
		// file that declares a provider is the finding.
		for _, line := range strings.Split(content, "\n") {
			trimmed := strings.TrimSpace(line)
			if !strings.HasPrefix(trimmed, "version") {
				continue
			}
			_, value, found := strings.Cut(trimmed, "=")
			if !found {
				continue
			}
			v := strings.TrimSpace(value)
			if v == `">= 0"` || v == `">=0"` || v == `""` {
				out = append(out, finding(
					r.ID(), Warning,
					"provider version constraint is open-ended ("+v+")",
					"pin to a concrete major version, e.g. \"~> 5.0\"",
					path,
				))
			}
		}
	}
	return out
}

// unpinnedModuleSource flags a module source that floats: a git source with no
// ref, or a registry source with no version.
type unpinnedModuleSource struct{}

func (r unpinnedModuleSource) ID() string { return "unpinned-module-source" }

func (r unpinnedModuleSource) Check(graph Graph, files map[string]string) []Finding {
	var out []Finding
	for _, n := range graph.Nodes {
		if n.BlockKind != BlockModule {
			continue
		}
		source := n.Attributes["source"]
		if source == "" {
			continue
		}
		literal := strings.Trim(source, "\"")
		if strings.HasPrefix(literal, "git::") || strings.Contains(literal, "github.com") {
			if !strings.Contains(literal, "ref=") {
				out = append(out, finding(
					r.ID(), Warning,
					"module "+n.Name+" uses a git source with no ref",
					"pin the module with ?ref=<tag or sha>",
					n.File,
				))
			}
			continue
		}
		// A registry or local path source. A registry source needs a version.
		if !strings.HasPrefix(literal, "./") && !strings.HasPrefix(literal, "../") {
			if _, ok := n.Attributes["version"]; !ok {
				out = append(out, finding(
					r.ID(), Warning,
					"module "+n.Name+" has no version constraint",
					"pin the module version, e.g. version = \"1.2.3\"",
					n.File,
				))
			}
		}
	}
	return out
}

// hardcodedLocation flags a location set to a literal string instead of a
// variable, so a multi-region deployment has to be edited in many places.
type hardcodedLocation struct{}

func (r hardcodedLocation) ID() string { return "hardcoded-location" }

func (r hardcodedLocation) Check(graph Graph, files map[string]string) []Finding {
	var out []Finding
	for _, n := range graph.Nodes {
		if n.BlockKind != BlockResource {
			continue
		}
		v, ok := n.Attributes["location"]
		if !ok || v == "" {
			continue
		}
		if !strings.HasPrefix(v, "\"") {
			continue
		}
		literal := strings.Trim(v, "\"")
		if isLocationLiteral(literal) {
			out = append(out, finding(
				r.ID(), Warning,
				n.Name+" hardcodes location = \""+literal+"\"",
				"set location from a variable so one edit moves the deployment",
				n.File,
			))
		}
	}
	return out
}

// itoa formats a non-negative int for message text.
func itoa(i int) string {
	return strconv.Itoa(i)
}

// isLocationLiteral reports whether a string looks like an Azure region name
// rather than an arbitrary literal. This keeps the rule from firing on every
// quoted string a resource happens to set.
func isLocationLiteral(s string) bool {
	l := strings.ToLower(s)
	regions := []string{
		"east us", "west us", "west europe", "north europe", "southeast asia",
		"east asia", "central us", "south central us", "west us 2", "west us 3",
		"uk south", "uk west", "japan east", "japan west", "australia east",
		"canada central", "canada east", "france central", "germany west central",
		"norway east", "switzerland north", "brazilsouth", "brazil south",
	}
	for _, r := range regions {
		if l == r {
			return true
		}
	}
	// A single-word region with no spaces that is not an expression.
	return !strings.ContainsAny(l, " ()")
}
