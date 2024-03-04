terraform {
  required_providers {
    aap = {
      source = "ansible/aap"
    }
  }
}

provider "aap" {
  host                 = "https://localhost:8043"
  username             = "test"
  password             = "test"
  insecure_skip_verify = true
}

resource "aap_host" "sample_foo" {
  inventory_id = 1
  name         = "tf_host_foo"
  variables = jsonencode(
    {
      "foo" : "bar"
    }
  )
  groups = [2, 3, 4]
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
  inventory_id = 1
  name         = "tf_host_bar"
  variables    = jsonencode(yamldecode(local.values_variables))
}

resource "aap_host" "sample_baz" {
  inventory_id = 1
  name         = "tf_host_baz"
  variables = jsonencode({
    foo = "bar"
    # Add other variables as needed
  })
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
