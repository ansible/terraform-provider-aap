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

# You can look up Job Templates by using either the `id` or a combination of `name` and `organization_name`.

# This example relies on a job template with id 7 that the user has access to.
data "aap_job_template" "sample_by_id" {
  id = 7
}

output "job_template_with_id" {
  value = data.aap_job_template.sample_by_id
}

data "aap_job_template" "sample_with_name_and_org_name" {
  name              = "Demo Job Template"
  organization_name = "Default"
}

output "job_template_with_name_and_org_name" {
  value = data.aap_job_template.sample_with_name_and_org_name
}
