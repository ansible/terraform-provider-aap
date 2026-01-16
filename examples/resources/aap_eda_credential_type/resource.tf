# Create a basic EDA credential type
resource "aap_eda_credential_type" "example" {
  name        = "my-credential-type"
  description = "My custom credential type"
}
