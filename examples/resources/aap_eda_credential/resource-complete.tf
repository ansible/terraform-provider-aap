# Define the credential type first
resource "aap_eda_credential_type" "github" {
  name        = "GitHub Token"
  description = "GitHub personal access token"

  inputs = jsonencode({
    fields = [
      {
        id     = "token"
        label  = "Personal Access Token"
        type   = "string"
        secret = true
      }
    ]
  })

  injectors = jsonencode({
    env = {
      GITHUB_TOKEN = "{{ token }}"
    }
  })
}

# Create credential using the type
resource "aap_eda_credential" "github" {
  name               = "my-github-credential"
  description        = "GitHub credential for automation"
  credential_type_id = aap_eda_credential_type.github.id
  organization_id    = 1

  # Secrets never stored in state
  # Version auto-increments when inputs change
  inputs_wo = jsonencode({
    token = var.github_token
  })
}
