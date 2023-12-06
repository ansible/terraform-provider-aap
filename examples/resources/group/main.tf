terraform {
  required_providers {
    aap = {
      source = "ansible/aap"
    }
  }
}

provider "aap" {
  host     = "https://controller.ansiblecloud.xyz/"
  username = "gomathiselvis"
  password = "Test123!"
  insecure_skip_verify = true
}

resource "aap_group" "sample" {
  id   = 1
  inventory = 2
  name = "tf_group" 
  variables = jsonencode(
    {
      "ansible_network_os": "iosxr"
    }
  )
}

output "group" {
  value = aap_group.sample
}
