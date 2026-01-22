resource "aap_eda_credential" "example" {
  name               = "my-api-credential"
  description        = "API credential for external service"
  credential_type_id = aap_eda_credential_type.api.id

  inputs_wo = jsonencode({
    username  = "service-account"
    api_token = var.api_token
  })
}
