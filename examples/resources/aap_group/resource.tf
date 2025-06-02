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

resource "aap_inventory" "my_inventory" {
  organization = 1
  name         = "A new inventory"
}

resource "aap_group" "sample_foo" {
  inventory_id = aap_inventory.my_inventory.id
  name         = "tf_group_foo"
  variables    = jsonencode({ "ansible_network_os" : "ios" })
}

locals {
  values_variables = <<EOT
exampleVariables:
  - name: "bar"
    namespace: "bar-namespace"
    type: 0
EOT
}

resource "aap_group" "sample_bar" {
  inventory_id = aap_inventory.my_inventory.id
  name         = "tf_group_bar"
  variables    = jsonencode(yamldecode(local.values_variables))
}

resource "aap_group" "sample_baz" {
  inventory_id = aap_inventory.my_inventory.id
  name         = "tf_group_baz"
  variables = jsonencode({
    ansible_network_os = "ios"
    # Add other variables as needed
  })
}

resource "aap_group" "sample_abc" {
  inventory_id = aap_inventory.my_inventory.id
  name         = "tf_group_abc"
  variables    = yamlencode({ "os" : "Linux", "automation" : "ansible" })
}

resource "aap_group" "sample_xyz" {
  inventory_id = aap_inventory.my_inventory.id
  name         = "tf_group_xyz"
  variables    = "os: Linux\nautomation: ansible-devel"
}

output "group_foo" {
  value = aap_group.sample_foo
}

output "group_bar" {
  value = aap_group.sample_bar
}

output "group_baz" {
  value = aap_group.sample_baz
}

output "group_abc" {
  value = aap_group.sample_abc
}

output "group_xyz" {
  value = aap_group.sample_xyz
}
