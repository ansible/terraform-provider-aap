data "aap_organization" "default" {
  name = "Default"
}

resource "aap_eda_project" "example" {
  name            = "My EDA Project"
  description     = "An example EDA project"
  url             = var.project_repo_url
  organization_id = data.aap_organization.default.id
}
