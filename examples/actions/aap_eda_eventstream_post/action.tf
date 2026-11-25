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

variable "event_stream_username" {
  type      = string
  sensitive = true
}

variable "event_stream_password" {
  type      = string
  sensitive = true
}

# Define an action to send a payload to Event-Driven Ansible via
# an event stream url. EDA must be configured with a rulebook activation
# to process these events.
action "aap_eda_eventstream_post" "create_auth_basic" {
  config {
    limit             = "infra"
    template_type     = "job"
    job_template_name = "After Create Job Template"
    organization_name = "Default"
    event_stream_config = {
      username = var.event_stream_username
      password = var.event_stream_password
      url      = "https://aap-event-stream-url.myaap.example.com/"
    }
  }
}

# Define action to send a payload with bearer token.
# Default behavior uses TFC_WORKLOAD_IDENTITY_TOKEN when
# workload identity is enabled and TFC_WORKLOAD_IDENTITY_AUDIENCE is set.
action "aap_eda_eventstream_post" "create_auth_bearer" {
  config {
    limit             = "infra"
    template_type     = "job"
    job_template_name = "After Create Job Template"
    organization_name = "Default"
    event_stream_config = {
      auth_type = "bearer"
      url       = "https://aap-event-stream-url.myaap.example.com/"
    }
  }
}

# Configure the actions to trigger after a resource is created
resource "terraform_data" "trigger" {
  input = "example"
  lifecycle {
    action_trigger {
      events  = [after_create]
      actions = [action.aap_eda_eventstream_post.create_auth_basic, action.aap_eda_eventstream_post.create_auth_bearer]
    }
  }
}
