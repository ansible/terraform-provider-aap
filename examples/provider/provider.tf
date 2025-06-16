# This example creates an inventory named `My new inventory`
# and adds a host `tf_host` and a group `tf_group` to it.
#
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

resource "aap_group" "my_group" {
  inventory_id = aap_inventory.my_inventory.id
  name         = "tf_group"
  variables = jsonencode(
    {
      "foo" : "bar"
    }
  )
}

resource "aap_host" "my_host" {
  inventory_id = aap_inventory.my_inventory.id
  name         = "tf_host"
  variables = jsonencode(
    {
      "foo" : "bar"
    }
  )
  groups = [aap_group.my_group.id]
}

resource "aap_job" "my_job" {
  job_template_id = 7
  inventory_id    = aap_inventory.my_inventory.id
}
