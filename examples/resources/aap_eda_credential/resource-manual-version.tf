# Create an EDA credential with manual version control
# You control when the credential updates by incrementing inputs_wo_version
resource "aap_eda_credential" "manual" {
  name               = "my-manual-credential"
  description        = "Credential with manual version control"
  credential_type_id = aap_eda_credential_type.api.id
  organization_id    = 1

  # Write-only credential inputs
  inputs_wo = jsonencode({
    username  = "service-account"
    api_token = var.api_token
  })

  # Increment this value to force credential update
  # The credential will only update when this value changes
  inputs_wo_version = 1
}
