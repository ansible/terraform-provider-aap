# Create an EDA credential type with inputs and injectors
resource "aap_eda_credential_type" "complete" {
  name        = "my-api-credential"
  description = "API credential type with username and token"

  inputs = jsonencode({
    fields = [
      {
        id    = "username"
        label = "Username"
        type  = "string"
      },
      {
        id     = "api_token"
        label  = "API Token"
        type   = "string"
        secret = true
      }
    ]
  })

  injectors = jsonencode({
    env = {
      API_USERNAME = "{{ username }}"
      API_TOKEN    = "{{ api_token }}"
    }
  })
}
