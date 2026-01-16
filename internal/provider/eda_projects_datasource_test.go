package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccEDAProjectsDataSource_all ensures the aap_eda_projects data source can retrieve
// all EDA projects without filters.
func TestAccEDAProjectsDataSource_all(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			skipTestWithoutEDAPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccEdaProjectsDataSourceConfig_all(),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.aap_eda_projects.test", "projects.#"),
				),
			},
		},
	})
}

// TestAccEDAProjectsDataSource_organizationFilter ensures the aap_eda_projects data source
// can filter projects by organization ID.
func TestAccEDAProjectsDataSource_organizationFilter(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			skipTestWithoutEDAPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccEdaProjectsDataSourceConfig_organizationFilter(),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.aap_eda_projects.test", "projects.#"),
				),
			},
		},
	})
}

// TestAccEDAProjectsDataSource_nameFilter ensures the aap_eda_projects data source
// can filter projects by name containing a string.
func TestAccEDAProjectsDataSource_nameFilter(t *testing.T) {
	rName := "tf-test-" + acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			skipTestWithoutEDAPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccEdaProjectsDataSourceConfig_nameFilter(rName),
				Check: resource.ComposeTestCheckFunc(
					// Check that we found at least 1 project
					resource.TestCheckResourceAttrSet("data.aap_eda_projects.test", "projects.#"),
					// Check that at least one project has the expected fields
					resource.TestCheckResourceAttrSet("data.aap_eda_projects.test", "projects.0.id"),
					resource.TestCheckResourceAttrSet("data.aap_eda_projects.test", "projects.0.name"),
				),
			},
		},
	})
}

func testAccEdaProjectsDataSourceConfig_all() string {
	return `
data "aap_eda_projects" "test" {
}
`
}

func testAccEdaProjectsDataSourceConfig_organizationFilter() string {
	return `
data "aap_organization" "test" {
  name = "Default"
}

data "aap_eda_projects" "test" {
  organization_id = data.aap_organization.test.id
}
`
}

func testAccEdaProjectsDataSourceConfig_nameFilter(name string) string {
	return fmt.Sprintf(`
data "aap_organization" "test" {
  name = "Default"
}

resource "aap_eda_project" "test" {
  name            = "%s"
  description     = "Test EDA project for data source"
  url             = "https://github.com/ansible/ansible-rulebook"
  organization_id = data.aap_organization.test.id
}

data "aap_eda_projects" "test" {
  name_contains = "%s"
  depends_on    = [aap_eda_project.test]
}
`, name, name)
}
