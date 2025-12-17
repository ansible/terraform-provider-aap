terraform {
  required_providers {
    aap = {
      source = "ansible/aap"
    }
  }
}

provider "aap" {
  host  = "https://myaap.example.com"
  token = "aap-token" # or set AAP_TOKEN
}

# Define an action to send a payload to AAP API.
action "aap_job_launch" "test" {
  config {
    job_template_id     = 1234
    wait_for_completion = true
  }
}

# Comprehensive action with all prompt on launch fields
action "aap_job_launch" "comprehensive" {
  config {
    job_template_id                     = 1234
    inventory_id                        = 5678
    extra_vars                          = jsonencode({ "environment" : "production" })
    limit                               = "webservers"
    job_tags                            = "deploy"
    skip_tags                           = "debug"
    show_changes                        = true
    verbosity                           = 1
    execution_environment               = 3
    forks                               = 5
    job_slice_count                     = 1
    timeout                             = 1800
    instance_groups                     = [1, 2]
    credentials                         = [10, 12]
    labels                              = [5, 7]
    wait_for_completion                 = true
    wait_for_completion_timeout_seconds = 300
  }
}

# Configure the action to trigger after a resource is created
resource "terraform_data" "trigger" {
  input = "example"
  lifecycle {
    action_trigger {
      events  = [after_create]
      actions = [action.aap_job_launch.test]
    }
  }
}
