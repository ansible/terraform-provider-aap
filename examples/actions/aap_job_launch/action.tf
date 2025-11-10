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
