resource "azurerm_subnet" "this" {
  name                 = var.name
  resource_group_name  = var.rgName
  virtual_network_name = var.virtual_network_name
  address_prefixes     = var.address_prefixes
}
