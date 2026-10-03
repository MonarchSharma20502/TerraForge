variable "name" {
  type        = string
  description = "Required: name"
}
variable "rgName" {
  type        = string
  description = "Required: rgName"
}
variable "location" {
  type        = string
  description = "Required: location"
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
