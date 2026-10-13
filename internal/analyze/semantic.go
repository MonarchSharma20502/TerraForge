// Package analyze: edge semantics. The legend the canvas draws from is fixed by
// the NOC governance template, so the semantic of a reference is derived from the
// attribute it passed through and the kind it points at - the same two signals
// the generator's canvas used, now read off real HCL.
package analyze

import "strings"

// SemanticFor decides the legend class of a reference.
//
// fromType is the type of the block carrying the reference, label is the
// attribute name it passed through, and toKind is the catalog kind of the target
// block ("" when the target is not an azurerm kind the catalog knows).
func SemanticFor(fromType, label, toKind string) EdgeSemantic {
	switch {
	case isVNetPeering(fromType, toKind):
		return SemanticPeering
	case isVNetLink(fromType, toKind):
		return SemanticVNetLink
	case isOnPrem(fromType, toKind):
		return SemanticOnPrem
	case isVNetIntegration(label, toKind):
		return SemanticVNetIntegration
	case isWebRequest(fromType):
		return SemanticWebRequest
	default:
		return SemanticDefault
	}
}

// isVNetPeering is a vnet pointing at another vnet: a peering or a vnet
// integration through a vnet_id.
func isVNetPeering(fromType, toKind string) bool {
	return kindOf(fromType) == "vnet" && toKind == "vnet"
}

// isVNetLink is a private DNS zone linked to a virtual network.
func isVNetLink(fromType, toKind string) bool {
	return kindOf(fromType) == "private_dns_zone" && toKind == "vnet"
}

// isOnPrem is a gateway connection, which the template draws as an on-premises
// tunnel.
func isOnPrem(fromType, toKind string) bool {
	return kindOf(fromType) == "virtual_network_gateway_connection" ||
		toKind == "virtual_network_gateway_connection"
}

// isVNetIntegration is a subnet or vnet reference from any resource: the
// template's blue connector.
func isVNetIntegration(label, toKind string) bool {
	if toKind == "subnet" || toKind == "vnet" {
		return true
	}
	l := strings.ToLower(label)
	return strings.Contains(l, "subnet_id") ||
		strings.Contains(l, "vnet_id") ||
		strings.Contains(l, "virtual_network_id")
}

// isWebRequest is a compute resource reaching a service it fronts.
func isWebRequest(fromType string) bool {
	switch kindOf(fromType) {
	case "windows_vm", "linux_vm", "windows_virtual_machine", "linux_virtual_machine",
		"virtual_machine", "linux_web_app", "windows_web_app":
		return true
	}
	return false
}

// kindOf is the catalog kind of a resource type when the classifier has one,
// else the trailing segment of the type after the provider prefix. It is used
// only for semantic matching, so a non-azurerm type compares against azurerm
// kinds and simply never matches.
func kindOf(resourceType string) string {
	if i := strings.IndexByte(resourceType, '_'); i > 0 {
		return resourceType[i+1:]
	}
	return resourceType
}
