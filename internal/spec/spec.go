// Package spec parses the Auto-nation DSL into the typed component graph.
//
// The DSL is JSON-Schema-backed and shaped to the house style: globals (location,
// subscription_id, name_config, tags) plus one nested object per resource family
// holding optional overrides - the same shape as the reference tfvars, so a
// generated Deployments/Dev/network.tfvars is valid input back into the generator.
package spec

import (
	"bytes"
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"

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

// ParseYAML parses a YAML spec document.
func ParseYAML(data []byte) (*ir.Blueprint, error) {
	var s Spec
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("spec: invalid yaml: %w", err)
	}
	return Parse(&s)
}

// ParseDocument parses a spec document in either JSON or YAML.
//
// The DSL is authored as YAML in examples and edited as JSON in the UI; both
// shapes decode into the same Spec.
func ParseDocument(data []byte) (*ir.Blueprint, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '{' {
		return ParseJSON(data)
	}
	return ParseYAML(data)
}

// buildComponent lifts one resource family entry into a component.
func buildComponent(kind, id string, props any) (*ir.Component, error) {
	c := &ir.Component{
		ID:         id,
		Kind:       kind,
		Stack:      stackFor(kind),
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
		"network_interface", "private_endpoint", "private_dns_zone",
		"application_gateway", "load_balancer", "firewall",
		"virtual_network_gateway", "virtual_wan", "virtual_hub",
		"nat_gateway", "public_ip_prefix", "application_security_group",
		"proximity_placement_group", "private_link_service", "dns_zone",
		"traffic_manager_profile", "cdn_frontdoor_profile",
		"cdn_frontdoor_endpoint", "cdn_frontdoor_origin_group",
		"cdn_frontdoor_origin", "cdn_frontdoor_route",
		"cdn_frontdoor_firewall_policy", "cdn_frontdoor_custom_domain",
		"cdn_frontdoor_custom_domain_association",
		"cdn_frontdoor_rule_set", "cdn_frontdoor_rule",
		"cdn_frontdoor_secret", "cdn_frontdoor_security_policy",
		"palo_alto_local_rulestack", "palo_alto_local_rulestack_rule",
		"palo_alto_next_generation_firewall_virtual_network_local_rulestack",
		"palo_alto_next_generation_firewall_virtual_hub_panorama",
		"palo_alto_virtual_network_appliance",
		"network_function_azure_traffic_collector",
		"virtual_network_peering", "virtual_network_gateway_connection",
		"network_security_rule", "virtual_network_dns_servers",
		"dns_cname_record", "dns_txt_record", "private_dns_cname_record",
		"private_dns_zone_virtual_network_link",
		"traffic_manager_azure_endpoint",
		"subnet_network_security_group_association",
		"subnet_route_table_association", "subnet_nat_gateway_association",
		"network_interface_security_group_association",
		"network_interface_application_security_group_association",
		"network_interface_backend_address_pool_association",
		"network_interface_nat_rule_association",
		"lb_backend_address_pool", "lb_probe", "lb_rule", "lb_nat_rule",
		"lb_nat_pool", "lb_outbound_rule",
		"firewall_application_rule_collection",
		"firewall_network_rule_collection",
		"nat_gateway_public_ip_association":
		return "Network"
	case "windows_vm", "linux_vm", "app_service_plan", "linux_web_app",
		"windows_web_app", "linux_function_app", "container_group",
		"kubernetes_cluster", "kubernetes_cluster_node_pool",
		"availability_set", "managed_disk", "image", "virtual_machine",
		"linux_virtual_machine_scale_set",
		"windows_virtual_machine_scale_set",
		"orchestrated_virtual_machine_scale_set",
		"virtual_machine_scale_set", "virtual_machine_extension",
		"virtual_machine_data_disk_attachment",
		"virtual_machine_scale_set_extension",
		"app_service_environment_v3", "static_web_app",
		"active_directory_domain_service", "arc_kubernetes_cluster",
		"function_app_function", "app_service_certificate",
		"app_service_certificate_order", "app_service_source_control",
		"virtual_desktop_host_pool", "virtual_desktop_workspace",
		"virtual_desktop_application_group",
		"virtual_desktop_workspace_application_group_association":
		return "Compute"
	case "storage_account", "key_vault", "log_analytics",
		"container_registry", "cosmosdb_account", "redis_cache",
		"managed_redis", "servicebus_namespace", "servicebus_queue",
		"servicebus_topic", "servicebus_subscription",
		"eventhub_namespace", "eventhub", "mssql_server",
		"mssql_database", "postgresql_flexible_server", "search_service",
		"databricks_workspace", "data_factory", "storage_container",
		"storage_share", "storage_queue",
		"storage_data_lake_gen2_filesystem",
		"storage_data_lake_gen2_path", "storage_blob",
		"netapp_account", "netapp_pool", "netapp_volume",
		"netapp_snapshot", "netapp_snapshot_policy",
		"netapp_backup_policy", "netapp_backup_vault",
		"netapp_account_encryption", "netapp_volume_bucket",
		"netapp_volume_bucket_with_server",
		"netapp_volume_group_oracle", "confidential_ledger",
		"batch_account", "batch_pool", "stream_analytics_job",
		"stream_analytics_reference_input_blob",
		"stream_analytics_stream_input_eventhub_v2", "iothub",
		"iothub_dps", "iothub_dps_certificate",
		"hdinsight_hadoop_cluster", "service_fabric_cluster",
		"logic_app_workflow", "logic_app_action_http",
		"logic_app_trigger_recurrence",
		"cosmosdb_postgresql_cluster", "data_lake_store",
		"data_lake_analytics_account", "data_lake_store_firewall_rule",
		"data_lake_analytics_firewall_rule",
		"data_factory_integration_runtime_self_hosted",
		"databricks_workspace_root_dbfs_customer_managed_key",
		"eventhub_cluster", "eventhub_consumer_group",
		"eventhub_authorization_rule",
		"eventhub_namespace_authorization_rule",
		"servicebus_namespace_authorization_rule",
		"servicebus_topic_authorization_rule",
		"mssql_firewall_rule", "mssql_failover_group",
		"mssql_server_extended_auditing_policy",
		"mssql_database_extended_auditing_policy",
		"mssql_managed_instance",
		"mssql_managed_instance_failover_group",
		"mssql_virtual_machine":
		return "Data"
	case "application_insights", "monitor_action_group",
		"monitor_metric_alert", "monitor_autoscale_setting",
		"monitor_data_collection_rule",
		"monitor_data_collection_rule_association",
		"monitor_private_link_scope", "log_analytics_solution",
		"monitor_diagnostic_setting":
		return "Monitoring"
	case "policy_definition", "management_group", "subscription",
		"security_center_subscription_pricing",
		"security_center_setting", "sentinel_watchlist",
		"sentinel_watchlist_item", "managed_application",
		"managed_application_definition", "template_deployment":
		return "Governance"
	case "automation_account", "automation_runbook",
		"automation_schedule", "automation_job_schedule":
		return "Automation"
	case "api_management", "api_management_api",
		"api_management_group", "api_management_product",
		"api_management_product_api", "api_management_product_group",
		"eventgrid_event_subscription",
		"bot_channels_registration", "bot_channel_directline":
		return "Integration"
	default:
		return "Core"
	}
}
