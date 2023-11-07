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

resource "aap_job" "sample" {
  job_template_id   = 9
  wait_for_completion = true
  wait_duration = 140
}

output "job_launch_url" {
  value = aap_job.sample.job_url
}
