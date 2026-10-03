variable "subscription_id" {
  type        = string
  description = "Azure subscription id"
}
variable "name" {
  type        = string
  description = "name for aml"
}
variable "address_prefixes" {
  type        = list(string)
  description = "Address prefixes for the subnet."
  default     = ["10.0.1.0/24"]
}
variable "location" {
  type        = string
  description = "location for spokevnet"
}
variable "address_space" {
  type        = list(string)
  description = "Address space for the virtual network."
  default     = ["10.0.0.0/16"]
}
variable "tags" {
  type        = map(string)
  description = "Virtual network tags merged with the stack tags."
  default     = {}
}
