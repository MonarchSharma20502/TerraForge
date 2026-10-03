module "aml" {
  source               = "../../Modules/Network/subnet"
  name                 = var.name
  rgName               = local.hub_name
  virtual_network_name = local.spokevnet_name
  address_prefixes     = var.address_prefixes
}
module "spokevnet" {
  source        = "../../Modules/Network/Vnet/virtualNetwork"
  name          = var.name
  rgName        = local.hub_name
  location      = var.location
  address_space = var.address_space
  tags          = var.tags
}
