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

resource "aap_job" "sample_foo" {
  job_template_id = 9
  inventory_id    = 2
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
  job_template_id = 9
  inventory_id    = 2
  extra_vars      = jsonencode(yamldecode(local.values_extra_vars))
}

resource "aap_job" "sample_baz" {
  job_template_id = 9
  inventory_id    = 2
  extra_vars = jsonencode({
    execution_environment_id = "3"
    # Add other variables as needed
  })
}

resource "aap_job" "sample_abc" {
  job_template_id = 9
  inventory_id    = 2
  extra_vars      = yamlencode({ "os" : "Linux", "automation" : "ansible" })
}

resource "aap_job" "sample_xyz" {
  job_template_id = 9
  inventory_id    = 2
  extra_vars      = "os: Linux\nautomation: ansible-devel"
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
