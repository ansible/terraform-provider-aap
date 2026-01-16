resource "aap_eda_project" "with_branch" {
  name            = "EDA Project with Branch"
  description     = "EDA project using specific branch"
  url             = var.project_repo_url
  scm_branch      = "main"
  organization_id = 1
}
