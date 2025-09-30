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

# You can look up Organizations by using either the `id` or their `name`.

# Look up organization by ID
data "aap_organization" "sample_by_id" {
  id = 7
}

output "organization_with_id" {
  value = data.aap_organization.sample_by_id
}

# Look up organization by name - this is the main use case for this data source
data "aap_organization" "sample_by_name" {
  name = "Default"
}

output "organization_with_name" {
  value = data.aap_organization.sample_by_name
}

# Example: Using the organization data source with an inventory resource
# This shows how to create an inventory in a specific organization by name
# instead of hard-coding the organization ID
resource "aap_inventory" "example" {
  name         = "My Inventory"
  organization = data.aap_organization.sample_by_name.id
  description  = "An inventory created using the organization data source"
}
