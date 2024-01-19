terraform {
  required_providers {
    aap = {
      source = "ansible/aap"
    }
  }
}

provider "aap" {
  host     = "https://controller.xxx.xyz/"
  username = "xxxx"
  password = "xxxx"
  insecure_skip_verify = true
}

resource "aap_group" "sample" {
  inventory_id = 1
  name = "tf_group" 
  variables = jsonencode({"ansible_network_os": "ios"})
}

output "group" {
  value = aap_group.sample
}
