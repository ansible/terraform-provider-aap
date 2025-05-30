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

# You can look up Workflow Job Templates by using either the `id` or a combination of `name` and `organization_name`.

# This example relies on a workflow job template with id 8 that the user has access to.
data "aap_workflow_job_template" "sample_by_id" {
  id = 8
}

output "workflow_job_template_with_id" {
  value = data.aap_workflow_job_template.sample_by_id
}

data "aap_workflow_job_template" "sample_with_name_and_org_name" {
  name              = "Demo Workflow Job Template"
  organization_name = "Default"
}

output "workflow_job_template_with_name_and_org_name" {
  value = data.aap_workflow_job_template.sample_with_name_and_org_name
}
