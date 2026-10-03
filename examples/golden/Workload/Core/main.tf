module "hub" {
  source   = "../../Modules/resourceGroup"
  name     = var.name
  location = var.location
  tags     = var.tags
}
