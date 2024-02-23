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

resource "aap_job" "sample" {
  job_template_id = 9
  inventory_id    = 2
  extra_vars      = jsonencode("{'resource_state' : 'absent'}")
  triggers = {
    "execution_environment_id" : "3"
  }
}

output "job" {
  value = aap_job.sample
}
