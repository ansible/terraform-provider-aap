resource "aap_eda_credential" "manual" {
  name               = "my-manual-credential"
  description        = "Credential with manual version control"
  credential_type_id = aap_eda_credential_type.api.id
  organization_id    = 1

  inputs_wo = jsonencode({
    username  = "service-account"
    api_token = var.api_token
  })

  inputs_wo_version = 1
}
