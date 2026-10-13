// Package analyze: the security rules. These are the rules that find code which
// is exploitable as written, so they are the only critical-severity rules apart
// from the naming and ingress ones.

package analyze

import (
	"strings"
)

// secretPatterns are the attribute-name fragments and literal keys that indicate
// a secret is being written into source. The list is deliberately broad: a false
// positive costs a read, a false negative ships a credential.
var secretPatterns = []string{
	"access_key", "accesskey", "connection_string", "connectionstring",
	"sas_token", "primary_key", "secondary_key", "account_key",
	"admin_password", "password", "secret", "client_secret", "api_key",
	"auth_token", "private_key",
}

// noHardcodedSecrets flags any secret-shaped string in any file, including in
// comments. A secret in a comment is a secret in the repo.
type noHardcodedSecrets struct{}

func (r noHardcodedSecrets) ID() string { return "no-hardcoded-secrets" }

func (r noHardcodedSecrets) Check(graph Graph, files map[string]string) []Finding {
	var out []Finding
	for path, content := range files {
		lower := strings.ToLower(content)
		for _, p := range secretPatterns {
			if !strings.Contains(lower, p) {
				continue
			}
			// A reference is not a secret: var.password or azurerm_key_vault.x.
			// Only literal assignments count, so the pattern must appear on a
			// line that assigns a quoted string rather than an expression.
			if !hasLiteralSecret(content, p) {
				continue
			}
			out = append(out, finding(
				r.ID(), Critical,
				"secret-shaped string "+p+" is hardcoded in source",
				"move it to a variable sourced from a secret store, not a literal",
				path,
			))
		}
	}
	return out
}

// hasLiteralSecret reports whether pattern appears on a line that assigns a
// quoted string. This is what keeps var.password and data.azurerm_key_vault...
// from tripping the rule while password = "hunter2" does.
func hasLiteralSecret(content, pattern string) bool {
	for _, line := range strings.Split(content, "\n") {
		lower := strings.ToLower(line)
		if !strings.Contains(lower, pattern) {
			continue
		}
		// An assignment of a quoted string is a literal. Anything else is an
		// expression reference and is safe.
		if _, value, found := strings.Cut(line, "="); found {
			trimmed := strings.TrimSpace(value)
			if strings.HasPrefix(trimmed, "\"") || strings.HasPrefix(trimmed, "<<") {
				return true
			}
		}
	}
	return false
}

// noInsecureTransport flags storage accounts and services that accept plain HTTP
// or an outdated TLS version.
type noInsecureTransport struct{}

func (r noInsecureTransport) ID() string { return "no-insecure-transport" }

func (r noInsecureTransport) Check(graph Graph, files map[string]string) []Finding {
	var out []Finding
	for _, n := range graph.Nodes {
		if n.BlockKind != BlockResource {
			continue
		}
		switch n.Kind {
		case "storage_account":
			if v, ok := n.Attributes["enable_https_traffic_only"]; ok && v == "false" {
				out = append(out, finding(
					r.ID(), Critical,
					n.Name+" accepts plain HTTP traffic (enable_https_traffic_only = false)",
					"set enable_https_traffic_only = true so only HTTPS is accepted",
					n.File,
				))
			}
			if v, ok := n.Attributes["min_tls_version"]; ok && isWeakTLS(v) {
				out = append(out, finding(
					r.ID(), Critical,
					n.Name+" permits an outdated TLS version ("+v+")",
					"set min_tls_version = TLS1_2 or higher",
					n.File,
				))
			}
		}
	}
	return out
}

// isWeakTLS reports whether a configured TLS version is below 1.2.
func isWeakTLS(v string) bool {
	switch strings.ToUpper(strings.TrimSpace(v)) {
	case "TLS1_0", "TLS1_1", "TLS1.0", "TLS1.1", "1.0", "1.1":
		return true
	}
	return false
}

// noVMPublicIP flags a virtual machine or network interface with a directly
// associated public IP, which exposes the workload instead of fronting it.
type noVMPublicIP struct{}

func (r noVMPublicIP) ID() string { return "no-vm-public-ip" }

func (r noVMPublicIP) Check(graph Graph, files map[string]string) []Finding {
	var out []Finding
	for _, n := range graph.Nodes {
		if n.BlockKind != BlockResource {
			continue
		}
		switch n.Kind {
		case "network_interface":
			if hasPublicIPRef(n, graph) {
				out = append(out, finding(
					r.ID(), Warning,
					n.Name+" has a public IP address associated",
					"front the VM with a load balancer or Application Gateway instead",
					n.File,
				))
			}
		case "windows_vm", "linux_vm", "virtual_machine":
			if v, ok := n.Attributes["public_ip_address_id"]; ok && v != "" && v != "null" {
				out = append(out, finding(
					r.ID(), Warning,
					n.Name+" has a public IP address associated",
					"front the VM with a load balancer or Application Gateway instead",
					n.File,
				))
			}
		}
	}
	return out
}

// hasPublicIPRef reports whether a network interface references a public IP.
func hasPublicIPRef(n Node, graph Graph) bool {
	for _, e := range graph.Edges {
		if e.From != n.ID {
			continue
		}
		if strings.Contains(strings.ToLower(e.Label), "public_ip") {
			return true
		}
	}
	return false
}
