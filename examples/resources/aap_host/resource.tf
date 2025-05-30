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

resource "aap_group" "group_1" {
  inventory_id = aap_inventory.my_inventory.id
  name         = "Group 1"
}

resource "aap_group" "group_2" {
  inventory_id = aap_inventory.my_inventory.id
  name         = "Group 2"
}

resource "aap_host" "sample_foo" {
  inventory_id = aap_inventory.my_inventory.id
  name         = "tf_host_foo"
  variables = jsonencode(
    {
      "foo" : "bar"
    }
  )
  groups = [aap_group.group_1.id, aap_group.group_2.id]
}

locals {
  values_variables = <<EOT
exampleVariables:
  - name: "bar"
    namespace: "bar-namespace"
    type: 0
EOT
}

resource "aap_host" "sample_bar" {
  inventory_id = aap_inventory.my_inventory.id
  name         = "tf_host_bar"
  variables    = jsonencode(yamldecode(local.values_variables))
}

resource "aap_host" "sample_baz" {
  inventory_id = aap_inventory.my_inventory.id
  name         = "tf_host_baz"
  variables = jsonencode({
    foo = "bar"
    # Add other variables as needed
  })
}

resource "aap_host" "sample_abc" {
  inventory_id = aap_inventory.my_inventory.id
  name         = "tf_host_abc"
  variables    = yamlencode({ "os" : "Linux", "automation" : "ansible" })
}

resource "aap_host" "sample_xyz" {
  inventory_id = aap_inventory.my_inventory.id
  name         = "tf_host_xyz"
  variables    = "os: Linux\nautomation: ansible-devel"
}

output "host_foo" {
  value = aap_host.sample_foo
}

output "host_bar" {
  value = aap_host.sample_bar
}

output "host_baz" {
  value = aap_host.sample_baz
}

output "host_abc" {
  value = aap_host.sample_abc
}
output "host_xyz" {
  value = aap_host.sample_xyz
}
