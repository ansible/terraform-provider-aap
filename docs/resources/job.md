---
page_title: "aap_job Resource - terraform-provider-aap"
description: |-
  Launches an AAP job.
  A job is launched only when the resource is first created or when the resource is changed. The triggers argument can be used to launch a new job based on any arbitrary value.
  This resource always creates a new job in AAP. A destroy will not delete a job created by this resource, it will only remove the resource from the state.
  Moreover, you can set wait_for_completion to true, then Terraform will wait until this job is created and reaches any final state before continuing. This parameter works in both create and update operations.
  You can also tweak wait_for_completion_timeout_seconds to control the timeout limit.
---

# aap_job (Resource)

Launches an AAP job.

A job is launched only when the resource is first created or when the resource is changed. The `triggers` argument can be used to launch a new job based on any arbitrary value.

This resource always creates a new job in AAP. A destroy will not delete a job created by this resource, it will only remove the resource from the state.

Moreover, you can set `wait_for_completion` to true, then Terraform will wait until this job is created and reaches any final state before continuing. This parameter works in both create and update operations.

You can also tweak `wait_for_completion_timeout_seconds` to control the timeout limit.

-> **Note** To pass an inventory to an aap_job resource, the underlying job template *must* have been configured to prompt for the inventory on launch.

!> **Warning** If an AAP Job launched by this resource is deleted from AAP, the resource will be removed from the state and a new job will be created to replace it.


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
  host     = "https://AAP_HOST"
  username = "ansible"
  password = "test123!"
}

resource "aap_inventory" "my_inventory" {
  name         = "A new inventory"
  organization = 1
}

resource "aap_job" "sample_foo" {
  job_template_id = 7
  inventory_id    = aap_inventory.my_inventory.id
  extra_vars      = jsonencode({ "resource_state" : "absent" })
  triggers = {
    "execution_environment_id" : "3"
  }
}

locals {
  values_extra_vars = <<EOT
exampleVariables:
  - name: "bar"
    namespace: "bar-namespace"
    type: 0
EOT
}

resource "aap_job" "sample_bar" {
  job_template_id = 7
  inventory_id    = aap_inventory.my_inventory.id
  extra_vars      = jsonencode(yamldecode(local.values_extra_vars))
}

resource "aap_job" "sample_baz" {
  job_template_id = 7
  inventory_id    = aap_inventory.my_inventory.id
  extra_vars = jsonencode({
    execution_environment_id = "3"
    # Add other variables as needed
  })
}

resource "aap_job" "sample_abc" {
  job_template_id = 7
  inventory_id    = aap_inventory.my_inventory.id
  extra_vars      = yamlencode({ "os" : "Linux", "automation" : "ansible" })
}

resource "aap_job" "sample_xyz" {
  job_template_id = 7
  inventory_id    = aap_inventory.my_inventory.id
  extra_vars      = "os: Linux\nautomation: ansible-devel"
}

resource "aap_job" "sample_wait_for_completion" {
  job_template_id                     = 7
  inventory_id                        = aap_inventory.my_inventory.id
  wait_for_completion                 = true
  wait_for_completion_timeout_seconds = 120
}

output "job_foo" {
  value = aap_job.sample_foo
}

output "job_bar" {
  value = aap_job.sample_bar
}

output "job_baz" {
  value = aap_job.sample_baz
}

output "job_abc" {
  value = aap_job.sample_abc
}

output "job_xyz" {
  value = aap_job.sample_xyz
}
```


## Ensuring Jobs Launch on Hosts created and Inventories updated in the same configuration

### Advanced Usage - `depends_on` in `aap_job` `resource` for `aap_host` `resource` creation
-> **Note** if you have HCL that creates an `aap_host` `resource` in an already existing `aap_inventory`, you will have to add a `depends_on` clause in the `aap_job` `resource` block of the `aap_job` that needs that `aap_host` to exist in the `aap_inventory` used for the `aap_job` creation.

If you do not use the depends_on clause as illustrated below you may run into a race condition where the job will attempt to launch before the inventory is updated with the host required.

### Example HCL for this scenario:

```terraform
data "aap_inventory" "inventory" {
  name              = "Demo Inventory"
  organization_name = "Default"
}

resource "aap_host" "host" {
  inventory_id = data.aap_inventory.inventory.id
  name         = "127.0.0.1"
}

data "aap_job_template" "job_template" {
  name              = "Demo Job Template"
  organization_name = "Default"
}

resource "aap_job" "job" {
  job_template_id = data.aap_job_template.job_template.id
  inventory_id    = data.aap_inventory.inventory.id

  # Force creation of this resource to wait for the aap_host.host resource to be created
  depends_on = [
    aap_host.host
  ]
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `job_template_id` (Number) Id of the job template.

### Optional

- `extra_vars` (String) Extra Variables. Must be provided as either a JSON or YAML string.
- `inventory_id` (Number) Identifier for the inventory where job should be created in. If not provided, the job will be created in the default inventory.
- `triggers` (Map of String) Map of arbitrary keys and values that, when changed, will trigger a creation of a new Job on AAP. Use 'terraform taint' if you want to force the creation of a new job without changing this value.
- `wait_for_completion` (Boolean) When this is set to `true`, Terraform will wait until this aap_job resource is created, reaches any final status and then, proceeds with the following resource operation
- `wait_for_completion_timeout_seconds` (Number) Sets the maximum amount of seconds Terraform will wait before timing out the updates, and the job creation will fail. Default value of `120`

### Read-Only

- `ignored_fields` (List of String) The list of properties set by the user but ignored on server side.
- `job_type` (String) Job type
- `status` (String) Status of the job
- `url` (String) URL of the job template
