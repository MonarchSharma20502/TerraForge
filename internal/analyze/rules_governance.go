// Package analyze: the well-architected rules. These are the rules that find
// code which deploys successfully but is missing the controls the NOC governance
// template expects: tagging, containment, backup, logging and locking.

package analyze

import (
	"regexp"
	"strconv"
	"strings"
)

// mandatoryTags flags a resource with no tags block at all, or one that omits
// the Environment and Owner tags the template mandates.
type mandatoryTags struct {
	c *Classifier
}

func (r mandatoryTags) ID() string { return "mandatory-tags" }

func (r mandatoryTags) Check(graph Graph, files map[string]string) []Finding {
	var out []Finding
	for _, n := range graph.Nodes {
		if n.BlockKind != BlockResource {
			continue
		}
		if !r.c.IsAzure(&n) {
			continue
		}
		// Resources that cannot take tags are exempt rather than being reported
		// as violations of a rule they cannot satisfy.
		if !taggable(n.Kind) {
			continue
		}
		tags := n.Attributes["tags"]
		if tags == "" {
			out = append(out, finding(
				r.ID(), Critical,
				n.Name+" ("+n.Type+") has no tags block",
				"add a tags block with at least Environment and Owner",
				n.File,
			))
			continue
		}
		if !tagsContain(tags, "Environment") {
			out = append(out, finding(
				r.ID(), Critical,
				n.Name+" ("+n.Type+") is missing the Environment tag",
				"add Environment = var.environment to the tags block",
				n.File,
			))
		}
		if !tagsContain(tags, "Owner") {
			out = append(out, finding(
				r.ID(), Critical,
				n.Name+" ("+n.Type+") is missing the Owner tag",
				"add Owner = var.owner to the tags block",
				n.File,
			))
		}
	}
	return out
}

// taggable reports whether a kind can carry a tags block. Association and
// policy resources are identified by their parent reference and take no tags.
func taggable(kind string) bool {
	switch kind {
	case "subnet_route_table_association", "subnet_network_security_group_association",
		"subnet_nat_gateway_association", "network_interface_security_group_association",
		"network_interface_application_security_group_association",
		"network_interface_backend_address_pool_association",
		"network_interface_nat_rule_association",
		"nat_gateway_public_ip_association",
		"virtual_desktop_workspace_application_group_association",
		"cdn_frontdoor_custom_domain_association",
		"mssql_database_extended_auditing_policy",
		"mssql_server_extended_auditing_policy",
		"key_vault_access_policy",
		"netapp_account_encryption",
		"databricks_workspace_root_dbfs_customer_managed_key",
		"virtual_machine_data_disk_attachment",
		"virtual_network_dns_servers",
		"security_center_setting",
		"security_center_subscription_pricing",
		"storage_data_lake_gen2_path",
		"automation_job_schedule",
		"app_service_source_control",
		"mssql_virtual_machine",
		"backup_protected_vm",
		"bot_channel_directline",
		"log_analytics_solution",
		"subscription",
		"api_management_product", "api_management_product_api",
		"api_management_product_group":
		return false
	}
	return true
}

// tagsContain reports whether a tags attribute source text contains a given key.
// The source text is matched rather than the parsed value because a tags block
// may be a dynamic expression the analyzer cannot evaluate, and a key that
// appears only in a comment should not satisfy the rule.
func tagsContain(tags, key string) bool {
	return strings.Contains(tags, key)
}

// noPublicIngress flags an NSG that allows inbound traffic, and a subnet that
// has no NSG attached at all. Both are the template's default-deny stance.
type noPublicIngress struct {
	c *Classifier
}

func (r noPublicIngress) ID() string { return "no-public-ingress-by-default" }

func (r noPublicIngress) Check(graph Graph, files map[string]string) []Finding {
	var out []Finding
	subnetHasNSG := map[string]bool{}
	for _, n := range graph.Nodes {
		if n.BlockKind != BlockResource {
			continue
		}
		switch n.Kind {
		case "nsg":
			if nsgAllowsInbound(n, graph) {
				out = append(out, finding(
					r.ID(), Critical,
					n.Name+" allows inbound traffic by default",
					"keep the NSG default-deny and add explicit allow rules only for the ports the workload needs",
					n.File,
				))
			}
		case "subnet":
			if hasNSGAttached(n, graph) {
				subnetHasNSG[n.ID] = true
			}
		}
	}
	for _, n := range graph.Nodes {
		if n.BlockKind != BlockResource || n.Kind != "subnet" {
			continue
		}
		if !subnetHasNSG[n.ID] {
			out = append(out, finding(
				r.ID(), Critical,
				n.Name+" has no network security group attached",
				"attach an NSG so the subnet is not reachable by default",
				n.File,
			))
		}
	}
	return out
}

// nsgAllowsInbound reports whether an NSG has an inbound allow rule. The rules
// are nested blocks, so the captured block source is scanned for the direction
// and access pair rather than reading a top-level attribute.
func nsgAllowsInbound(n Node, graph Graph) bool {
	src := n.Blocks["security_rule"]
	if src == "" {
		return false
	}
	lines := strings.Split(src, "\n")
	inbound := false
	allows := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "direction") && strings.Contains(lower, "inbound") {
			inbound = true
		}
		if strings.HasPrefix(lower, "access") && strings.Contains(lower, "allow") {
			allows = true
		}
	}
	return inbound && allows
}

// hasNSGAttached reports whether a subnet has an NSG association edge.
func hasNSGAttached(n Node, graph Graph) bool {
	for _, e := range graph.Edges {
		if e.From != n.ID {
			continue
		}
		if strings.Contains(strings.ToLower(e.Label), "network_security_group") {
			return true
		}
		if strings.Contains(strings.ToLower(e.To), "network_security_group") {
			return true
		}
	}
	return false
}

// namingConformance checks real resource names against the Azure pattern and the
// kind's strict-name bounds. This is the one rule that uses the catalog's
// strictName data, because the names here are the ones the user typed.
type namingConformance struct {
	c *Classifier
}

func (r namingConformance) ID() string { return "naming-conformance" }

func (r namingConformance) Check(graph Graph, files map[string]string) []Finding {
	var out []Finding
	pattern := regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)+$`)
	for _, n := range graph.Nodes {
		if n.BlockKind != BlockResource {
			continue
		}
		entry, ok := r.c.byType[n.Type]
		if !ok || entry.StrictName == nil {
			// A name that is not strict still has to match the house pattern.
			name := n.Attributes["name"]
			if name == "" || !strings.HasPrefix(name, "\"") {
				continue
			}
			literal := strings.Trim(name, "\"")
			if !pattern.MatchString(literal) {
				out = append(out, finding(
					r.ID(), Warning,
					n.Name+" name "+literal+" does not match the naming pattern (lowercase, dash-separated)",
					"use the <abbr>-<bu>-<platform>-<env>-<location>-<nn> pattern",
					n.File,
				))
			}
			continue
		}
		name := n.Attributes["name"]
		if name == "" || !strings.HasPrefix(name, "\"") {
			continue
		}
		literal := strings.Trim(name, "\"")
		check := literal
		if entry.StrictName.AlnumOnly || entry.StrictName.StripDashes {
			check = strings.ReplaceAll(check, "-", "")
		}
		if entry.StrictName.Min > 0 && len(check) < entry.StrictName.Min {
			out = append(out, finding(
				r.ID(), Warning,
				n.Name+" name "+literal+" is "+strconv.Itoa(len(check))+" characters, below the "+strconv.Itoa(entry.StrictName.Min)+" minimum for "+n.Type,
				"lengthen the name to at least "+strconv.Itoa(entry.StrictName.Min)+" characters",
				n.File,
			))
		}
		if entry.StrictName.Max > 0 && len(check) > entry.StrictName.Max {
			out = append(out, finding(
				r.ID(), Warning,
				n.Name+" name "+literal+" is "+strconv.Itoa(len(check))+" characters, above the "+strconv.Itoa(entry.StrictName.Max)+" maximum for "+n.Type,
				"shorten the name to at most "+strconv.Itoa(entry.StrictName.Max)+" characters",
				n.File,
			))
		}
	}
	return out
}
