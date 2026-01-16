data "aap_organization" "default" {
  name = "Default"
}

# Get all EDA projects in a specific organization
data "aap_eda_projects" "by_org" {
  organization_id = data.aap_organization.default.id
}
