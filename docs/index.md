---
page_title: "AAP Provider"
subcategory: ""
description: |-
  Terraform provider for Ansible Automation Platform (AAP).
---

# AAP Provider

The AAP Provider allows Terraform to manage Ansible Automation Platform resources.

Use the navigation to the left to read about the available resources.


## Example Usage

```terraform
terraform {
  required_providers {
    aap = {
      source = "ansible/aap"
    }
  }
}

provider "aap" {
  host     = "https://localhost:8043"
  username = "ansible"
  password = "test123!"
  insecure_skip_verify = true
}

resource "aap_inventory" "my_inventory" {
  name = "My new inventory"
  description = "A new inventory for testing"
  variables = jsonencode(
    {
      "foo": "bar"
    }
  )
}

resource "aap_host" "my_host" {
  inventory_id = aap_inventory.my_inventory.id
  name = "tf_host"
  variables = jsonencode(
    {
      "foo": "bar"
    }
  )
  groups = [2, 3, 4]
}

resource "aap_group" "my_group" {
  inventory_id = aap_inventory.my_inventory.id
  name = "tf_group" 
  variables = jsonencode(
    {
      "foo": "bar"
    }
  )
}
```
