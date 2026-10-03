variable "name" {
  type        = string
  description = "Required: name"
}
variable "rgName" {
  type        = string
  description = "Required: rgName"
}
variable "virtual_network_name" {
  type        = string
  description = "Required: virtual_network_name"
}
variable "address_prefixes" {
  type        = list(string)
  description = "Address prefixes for the subnet."
  default     = ["10.0.1.0/24"]
}
