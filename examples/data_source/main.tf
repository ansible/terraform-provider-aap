terraform {
  required_providers {
    aap = {
      source = "github.com/ansible/aap"
    }
  }
}

provider "aap" {
  host     = "https://localhost:8043"
  username = "ansible"
  password = "test123!"
  insecure_skip_verify = true
}

variable "state_id" {
  type = number
  description = "The id of the state to request"
}

data "aap_inventory" "sample" {
  path = "/api/v2/state/${var.state_id}/"
}

output "inventory_hosts" {
  value = data.aap_inventory.sample.hosts
}

output "inventory_groups" {
  value = data.aap_inventory.sample.groups
}

