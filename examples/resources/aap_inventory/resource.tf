terraform {
  required_providers {
    aap = {
      source = "ansible/aap"
    }
  }
}

provider "aap" {
  host     = "https://AAP_HOST"
  username = "ansible"
  password = "test123!"
}

resource "aap_inventory" "sample_foo" {
  name         = "My new inventory foo"
  description  = "A new inventory for testing"
  organization = 1
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
  name         = "My new inventory bar"
  description  = "A new inventory for testing"
  organization = 1
  variables    = jsonencode(yamldecode(local.values_variables))
}

resource "aap_inventory" "sample_baz" {
  name         = "My new inventory baz"
  description  = "A new inventory for testing"
  organization = 1
  variables = jsonencode({
    foo = "bar"
    # Add other variables as needed
  })
}

resource "aap_inventory" "sample_abc" {
  name         = "My new inventory abc"
  description  = "A new inventory for testing"
  organization = 1
  variables    = yamlencode({ "os" : "Linux", "automation" : "ansible" })
}

resource "aap_inventory" "sample_xyz" {
  name         = "My new inventory xyz"
  description  = "A new inventory for testing"
  organization = 1
  variables    = "os: Linux\nautomation: ansible-devel"
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

output "inventory_abc" {
  value = aap_inventory.sample_abc
}

output "inventory_xyz" {
  value = aap_inventory.sample_xyz
}
