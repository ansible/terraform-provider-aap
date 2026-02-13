resource "aap_eda_project" "with_proxy" {
  name            = "EDA Project with Proxy"
  description     = "EDA project using proxy server"
  url             = var.project_repo_url
  proxy           = "http://proxy.example.com:8080"
  organization_id = 1
}
