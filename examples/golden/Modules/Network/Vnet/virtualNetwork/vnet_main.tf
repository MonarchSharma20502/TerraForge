resource "azurerm_virtual_network" "this" {
  name                = var.name
  resource_group_name = var.rgName
  location            = var.location
  address_space       = var.address_space
  tags                = var.tags
}
