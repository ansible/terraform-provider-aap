# Create an EDA credential with write-only secret inputs
# Version is auto-managed by default - increments when inputs_wo changes
resource "aap_eda_credential" "example" {
  name               = "my-api-credential"
  description        = "API credential for external service"
  credential_type_id = aap_eda_credential_type.api.id

  # Write-only: sent to API but NEVER stored in Terraform state
  inputs_wo = jsonencode({
    username  = "service-account"
    api_token = var.api_token
  })
}
