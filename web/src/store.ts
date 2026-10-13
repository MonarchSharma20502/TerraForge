// The single store for the analyzer. The input files are the source of truth;
// the graph, the diagrams and the recommendations are all pure functions of
// them, recomputed by the wasm core on every edit.

import { useCallback, useEffect, useState } from "react";

import { Core, loadCore, type AnalyzeResult, type CatalogEntry } from "./core/wasm";

export interface AnalyzerState {
  // files are the Terraform being analyzed, keyed by path.
  files: Record<string, string>;
  // selection is the node id the recommendations and file viewer focus on.
  selection: string | null;
  // catalog labels nodes the analyzer recognized.
  catalog: CatalogEntry[];
  // result is the last analysis, or null before the core loads.
  result: AnalyzeResult | null;
  // coreError is set when the wasm core itself failed to load.
  coreError: string | null;
  // loading is true until the wasm core has instantiated.
  loading: boolean;
}

const EXAMPLE = `terraform {
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 5.0"
    }
  }
}

provider "azurerm" {
  features {}
}

variable "location" {
  default = "eastus"
}

resource "azurerm_resource_group" "hub" {
  name     = "rg-hub-prod-sea-01"
  location = var.location
  tags = {
    Environment = "Production"
    Owner       = "platform-team"
  }
}

resource "azurerm_virtual_network" "spoke" {
  name                = "vnet-spoke-prod-sea-01"
  location            = var.location
  resource_group_name = azurerm_resource_group.hub.name
  address_space       = ["10.0.0.0/16"]
}

resource "azurerm_subnet" "aml" {
  name                 = "subnet-aml-prod-sea-01"
  resource_group_name  = azurerm_resource_group.hub.name
  virtual_network_name = azurerm_virtual_network.spoke.name
  address_prefixes     = ["10.0.1.0/24"]
}

resource "azurerm_network_security_group" "open" {
  name                = "nsg-open-prod-sea-01"
  location            = var.location
  resource_group_name = azurerm_resource_group.hub.name

  security_rule {
    name                       = "allow-all"
    priority                   = 100
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = "*"
    source_address_prefix      = "*"
    destination_address_prefix = "*"
  }
}

resource "azurerm_network_interface" "nic" {
  name                = "nic-vm-prod-sea-01"
  location            = var.location
  resource_group_name = azurerm_resource_group.hub.name

  ip_configuration {
    name                          = "internal"
    subnet_id                     = azurerm_subnet.aml.id
    private_ip_address_allocation = "Dynamic"
  }
}

resource "azurerm_linux_virtual_machine" "vm" {
  name                            = "vm-app-prod-sea-01"
  location                        = var.location
  resource_group_name             = azurerm_resource_group.hub.name
  network_interface_ids           = [azurerm_network_interface.nic.id]
  size                            = "Standard_B2s"
  admin_username                  = "adminuser"
  disable_password_authentication = false

  os_disk {
    caching              = "ReadWrite"
    storage_account_type = "Standard_LRS"
  }

  source_image_reference {
    publisher = "Canonical"
    offer     = "0001-com-ubuntu-server-jammy"
    sku       = "22_04-lts-gen2"
    version   = "latest"
  }
}
`;

export function useAnalyzer(): AnalyzerState & {
  setFiles: (files: Record<string, string>) => void;
  addFiles: (files: Record<string, string>) => void;
  editFile: (path: string, content: string) => void;
  removeFile: (path: string) => void;
  select: (nodeId: string | null) => void;
  loadExample: () => void;
  clear: () => void;
} {
  const [state, setState] = useState<AnalyzerState>({
    files: {},
    selection: null,
    catalog: [],
    result: null,
    coreError: null,
    loading: true,
  });
  const [core, setCore] = useState<Core | null>(null);

  // Load the wasm core once, then pull the catalog for node labels.
  useEffect(() => {
    let cancelled = false;
    loadCore()
      .then((c) => {
        if (cancelled) return;
        setCore(c);
        setState((s) => ({ ...s, catalog: c.catalog(), loading: false }));
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setState((s) => ({
          ...s,
          coreError: err instanceof Error ? err.message : String(err),
          loading: false,
        }));
      });
    return () => {
      cancelled = true;
    };
  }, []);

  // Re-analyze whenever the input changes and the core is ready. This is what
  // makes the canvas live: every edit flows through the same pipeline the CLI
  // runs, in the browser, with no backend.
  useEffect(() => {
    if (!core) return;
    const result = core.analyze(state.files);
    setState((s) => ({ ...s, result }));
  }, [core, state.files]);

  const setFiles = useCallback((files: Record<string, string>) => {
    setState((s) => ({ ...s, files, selection: null }));
  }, []);

  const addFiles = useCallback((incoming: Record<string, string>) => {
    setState((s) => ({ ...s, files: { ...s.files, ...incoming } }));
  }, []);

  const editFile = useCallback((path: string, content: string) => {
    setState((s) => ({ ...s, files: { ...s.files, [path]: content } }));
  }, []);

  const removeFile = useCallback((path: string) => {
    setState((s) => {
      const files = { ...s.files };
      delete files[path];
      return { ...s, files };
    });
  }, []);

  const select = useCallback((nodeId: string | null) => {
    setState((s) => ({ ...s, selection: nodeId }));
  }, []);

  const loadExample = useCallback(() => {
    setState((s) => ({ ...s, files: { "main.tf": EXAMPLE }, selection: null }));
  }, []);

  const clear = useCallback(() => {
    setState((s) => ({ ...s, files: {}, selection: null }));
  }, []);

  return {
    ...state,
    setFiles,
    addFiles,
    editFile,
    removeFile,
    select,
    loadExample,
    clear,
  };
}
