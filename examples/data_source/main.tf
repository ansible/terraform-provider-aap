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

resource "aap_inventory" "my_inventory" {
  name = "My new inventory"
  description = "A new inventory for testing"
  variables = jsonencode(
    {
      "foo": "bar"
    }
  )
}

output "inventory" {
  value = aap_inventory.my_inventory
}

data "aap_inventory" "sample" {
  id = aap_inventory.my_inventory.id
}

output "inventory_details" {
  value = data.aap_inventory.sample
}
output "inventory_variables" {
  value = data.aap_inventory.sample.variables
}
