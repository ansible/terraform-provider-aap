---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "aap_group Resource - terraform-provider-aap"
subcategory: ""
description: |-
  
---

# aap_group (Resource)





<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `inventory_id` (Number) Inventory id
- `name` (String) Name of the group

### Optional

- `description` (String) Description for the group
- `variables` (String) Variables for the group configuration. Must be provided as either a JSON or YAML string.

### Read-Only

- `id` (Number) Group Id
- `url` (String) URL for the group
