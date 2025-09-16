# This example creates an inventory named `My new inventory`
# and adds a host `tf_host` and a group `tf_group` to it,
# and then launches a job based on the "Demo Job Template"
# in the "Default" organization using the inventory created.
#
terraform {
  required_providers {
    aap = {
      source = "ansible/aap"
    }
  }
}

provider "aap" {
  host = "https://AAP_HOST" # Also supports AAP_HOSTNAME environment variable

  # Token authentication is recommended
  token = "my-aap-token" # Also supports AAP_TOKEN environment variable

  # Basic authentication is also supported, ignored if token is set
  username = "my-aap-username" # Also supports AAP_USERNAME environment variable
  password = "my-aap-password" # Also supports AAP_PASSWORD environment variable
}

resource "aap_inventory" "my_inventory" {
  name         = "My new inventory"
  description  = "A new inventory for testing"
  organization = 1
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

data "aap_job_template" "demo_job_template" {
  name              = "Demo Job Template"
  organization_name = "Default"
}

# In order for passing the inventory id to the job template execution, the Inventory on the job template needs to be set to "prompt on launch"
resource "aap_job" "my_job" {
  inventory_id    = aap_inventory.my_inventory.id
  job_template_id = aap_job_template.demo_job_template.id

  # This resource creation needs to wait for the host and group to be created in the inventory
  depends_on = [
    aap_host.my_host,
    aap_group.my_group
  ]
}
