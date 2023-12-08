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
  inventory_id = 1
  name = "tf_group" 
}

output "group" {
  value = aap_group.sample
}
