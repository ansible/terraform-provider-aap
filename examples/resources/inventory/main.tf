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
  name        = "My new inventory"
  description = "A new inventory for testing"
  variables = jsonencode(
    {
      "foo" : "bar"
    }
  )
}

output "inventory" {
  value = aap_inventory.my_inventory
}
