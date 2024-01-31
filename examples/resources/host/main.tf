terraform {
  required_providers {
    aap = {
      source = "ansible/aap"
    }
  }
}

provider "aap" {
  host     = "https://localhost:8043"
  username = "test"
  password = "test"
  insecure_skip_verify = true
}

resource "aap_host" "sample" {
  inventory_id = 1
  name = "tf_host"
  variables = jsonencode(
    {
      "foo": "bar"
    }
  )
  groups = [2, 3, 4]
}

output "host" {
  value = aap_host.sample
}
