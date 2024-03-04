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

resource "aap_group" "sample_foo" {
  inventory_id = 1
  name         = "tf_group_foo"
  variables    = jsonencode({"ansible_network_os" : "ios"})
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
  inventory_id = 1
  name         = "tf_group_bar"
  variables    = jsonencode(yamldecode(local.values_variables))
}

resource "aap_group" "sample_baz" {
  inventory_id = 1
  name         = "tf_group_baz"
  variables    = jsonencode({
    ansible_network_os = "ios"
    # Add other variables as needed
  })
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
