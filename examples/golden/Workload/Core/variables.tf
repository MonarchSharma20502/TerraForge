variable "subscription_id" {
  type        = string
  description = "Azure subscription id"
}
variable "name" {
  type        = string
  description = "name for hub"
}
variable "location" {
  type        = string
  description = "location for hub"
}
variable "tags" {
  type        = map(string)
  description = "Resource group tags merged with the stack tags."
  default     = {}
}
