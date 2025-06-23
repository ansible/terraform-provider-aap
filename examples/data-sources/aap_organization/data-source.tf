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

# This example relies on a job template with id 7 that the user has access to.
data "aap_organization" "sample_by_id" {
  id = 7
}

output "organization_with_id" {
  value = data.aap_organization.sample_by_id
}

data "aap_organization" "sample_with_name_and_org_name" {
  name = "Default"
}

output "organization_with_name" {
  value = data.aap_organization.sample_with_name_and_org_name
}
