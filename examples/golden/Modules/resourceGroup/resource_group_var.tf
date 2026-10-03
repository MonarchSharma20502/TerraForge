variable "name" {
  type        = string
  description = "Required: name"
}
variable "location" {
  type        = string
  description = "Required: location"
}
variable "tags" {
  type        = map(string)
  description = "Resource group tags merged with the stack tags."
  default     = {}
}
