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

resource "aap_workflow_job" "sample_foo" {
  workflow_job_template_id = 8
  inventory_id             = aap_inventory.my_inventory.id
}

locals {
  values_extra_vars = <<EOT
exampleVariables:
  - name: "bar"
    namespace: "bar-namespace"
    type: 0
EOT
}

resource "aap_workflow_job" "sample_bar" {
  workflow_job_template_id = 8
  extra_vars               = jsonencode(yamldecode(local.values_extra_vars))
}

resource "aap_workflow_job" "sample_baz" {
  workflow_job_template_id = 8
  extra_vars = jsonencode({
    execution_environment_id = "3"
    # Add other variables as needed
  })
}

resource "aap_workflow_job" "sample_abc" {
  workflow_job_template_id = 8
  inventory_id             = aap_inventory.my_inventory.id
  extra_vars               = yamlencode({ "os" : "Linux", "automation" : "ansible" })
}

resource "aap_workflow_job" "sample_xyz" {
  workflow_job_template_id = 8
  inventory_id             = aap_inventory.my_inventory.id
  extra_vars               = "os: Linux\nautomation: ansible-devel"
}

output "job_foo" {
  value = aap_workflow_job.sample_foo
}

output "job_bar" {
  value = aap_workflow_job.sample_bar
}

output "job_baz" {
  value = aap_workflow_job.sample_baz
}

output "job_abc" {
  value = aap_workflow_job.sample_abc
}

output "job_xyz" {
  value = aap_workflow_job.sample_xyz
}
