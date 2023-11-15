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

locals {
  extra_vars = {
    "simple": "testing",
    "simple_bool": false,
    "simple_object": {
        "key1": "value1",
        "key2": "value2"
    }
  }
}

resource "aap_job" "sample" {
  job_template_id   = 10
  inventory_id = 1
  wait_for_completion = false
  wait_duration = 10
  extra_vars = jsonencode(locals.extra_vars)
}

output "job_launch_url" {
  value = aap_job.sample.job_url
}
