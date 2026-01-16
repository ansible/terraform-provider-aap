# Get EDA projects with names containing "production"
data "aap_eda_projects" "production" {
  name_contains = "production"
}
