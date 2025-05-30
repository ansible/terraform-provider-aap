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

# You can look up Inventories by using either the `id` or a combination of `name` and `organization_name`.

data "aap_inventory" "sample_by_id" {
  id = 1
}

output "inventory_details_with_id" {
  value = data.aap_inventory.sample_by_id
}

data "aap_inventory" "sample_with_name_and_org_name" {
  name              = "Demo Inventory"
  organization_name = "Default"
}

output "inventory_details_with_name_and_org_name" {
  value = data.aap_inventory.sample_with_name_and_org_name
}
