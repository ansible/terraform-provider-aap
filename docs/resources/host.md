---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "aap_host Resource - terraform-provider-aap"
subcategory: ""
description: |-
  
---

# aap_host (Resource)





<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `inventory_id` (Number) Inventory id
- `name` (String) Name of the host

### Optional

- `description` (String) Description for the host
- `enabled` (Boolean) Denotes if the host is online and is available
- `groups` (Set of Number) The list of groups to assosicate with a host.
- `variables` (String) Variables for the host configuration. Must be provided as either a JSON or YAML string.

### Read-Only

- `id` (Number) Id of the host
- `url` (String) URL of the host
