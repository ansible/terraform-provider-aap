terraform {
  required_providers {
    aap = {
      source = "ansible/aap"
    }
  }
}

provider "aap" {
  host                 = "https://localhost:8043"
  username             = "ansible"
  password             = "test123!"
  insecure_skip_verify = true
}

variable "inventory_id" {
   type = number
   description = "The inventory id"
}

data "aap_inventory" "sample" {
  id = var.inventory_id
}

output "inventory_details" {
  value = data.aap_inventory.sample
}
