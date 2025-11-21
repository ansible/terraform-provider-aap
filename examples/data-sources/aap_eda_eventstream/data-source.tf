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

# Look up an EDA Event Stream by name.
data "aap_eda_eventstream" "example" {
  name = "my-event-stream"
}

output "eventstream_details" {
  value = data.aap_eda_eventstream.example
}

# Example: Display specific attributes of the event stream
output "eventstream_id" {
  value = data.aap_eda_eventstream.example.id
}

output "eventstream_url" {
  value = data.aap_eda_eventstream.example.url
}