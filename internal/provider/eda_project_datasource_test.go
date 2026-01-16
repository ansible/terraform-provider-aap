package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestNewEDAProjectDataSource(t *testing.T) {
	testDataSource := NewEDAProjectDataSource()

	expectedMetadataEntitySlug := "eda_project"
	expectedDescriptiveEntityName := "EDA Project"
	expectedAPIEntitySlug := "projects"

	switch v := testDataSource.(type) {
	case *EDAProjectDataSource:
		if v.APIEntitySlug != expectedAPIEntitySlug {
			t.Errorf("Incorrect APIEntitySlug. Got: %s, wanted: %s", v.APIEntitySlug, expectedAPIEntitySlug)
		}
		if v.DescriptiveEntityName != expectedDescriptiveEntityName {
			t.Errorf("Incorrect DescriptiveEntityName. Got: %s, wanted: %s", v.DescriptiveEntityName, expectedDescriptiveEntityName)
		}
		if v.MetadataEntitySlug != expectedMetadataEntitySlug {
			t.Errorf("Incorrect MetadataEntitySlug. Got: %s, wanted: %s", v.MetadataEntitySlug, expectedMetadataEntitySlug)
		}
	default:
		t.Errorf("Incorrect datasource type returned. Got: %T, wanted: %T", v, testDataSource)
	}
}

// TestAccEDAProjectDataSource ensures the aap_eda_project data source can retrieve
// an EDA project successfully.
func TestAccEDAProjectDataSource(t *testing.T) {
	rName := "tf-test-" + acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			skipTestWithoutEDAPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccEdaProjectDataSourceConfig(rName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.aap_eda_project.test", "name", rName),
					resource.TestCheckResourceAttrSet("data.aap_eda_project.test", "id"),
					resource.TestCheckResourceAttrSet("data.aap_eda_project.test", "url"),
				),
			},
		},
	})
}

func testAccEdaProjectDataSourceConfig(name string) string {
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

data "aap_eda_project" "test" {
  name = aap_eda_project.test.name
}
`, name)
}
