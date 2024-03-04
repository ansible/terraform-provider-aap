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

resource "aap_inventory" "sample_foo" {
  name        = "My new inventory foo"
  description = "A new inventory for testing"
  variables = jsonencode(
    {
      "foo" : "bar"
    }
  )
}

locals {
  values_variables = <<EOT
exampleVariables:
  - name: "bar"
    namespace: "bar-namespace"
    type: 0
EOT
}

resource "aap_inventory" "sample_bar" {
  name        = "My new inventory bar"
  description = "A new inventory for testing"
  variables   = jsonencode(yamldecode(local.values_extra_vars))
}

resource "aap_inventory" "sample_baz" {
  name        = "My new inventory baz"
  description = "A new inventory for testing"
  variables = jsonencode({
    foo = "bar"
    # Add other variables as needed
  })
}

output "inventory_foo" {
  value = aap_inventory.sample_foo
}

output "inventory_bar" {
  value = aap_inventory.sample_bar
}

output "inventory_baz" {
  value = aap_inventory.sample_baz
}
