package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestNewEDACredentialTypeDataSource(t *testing.T) {
	testDataSource := NewEDACredentialTypeDataSource()

	switch v := testDataSource.(type) {
	case *EDACredentialTypeDataSource:
	default:
		t.Errorf("Incorrect datasource type returned. Got: %T, wanted: *EDACredentialTypeDataSource", v)
	}
}

func TestAccEDACredentialTypeDataSource(t *testing.T) {
	randomName := acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)
	description := "Test credential type description"

	expectedInputs := `{"fields":[{"id":"username","type":"string","label":"Username"},{"id":"password","type":"string","label":"Password","secret":true}]}`
	expectedInjectors := `{"env":{"MY_PASSWORD":"{{ password }}","MY_USERNAME":"{{ username }}"}}`

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccEDACredentialTypeDataSourceByName(randomName, description),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.aap_eda_credential_type.test", "name", randomName),
					resource.TestCheckResourceAttr("data.aap_eda_credential_type.test", "description", description),
					resource.TestCheckResourceAttrSet("data.aap_eda_credential_type.test", "id"),
					resource.TestCheckResourceAttr("data.aap_eda_credential_type.test", "inputs", expectedInputs),
					resource.TestCheckResourceAttr("data.aap_eda_credential_type.test", "injectors", expectedInjectors),
				),
			},
		},
	})
}

func TestAccEDACredentialTypeDataSourceByID(t *testing.T) {
	randomName := acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)
	description := "Test credential type description for ID lookup"

	expectedInputs := `{"fields":[{"id":"api_key","type":"string","label":"API Key","secret":true}]}`
	expectedInjectors := `{"env":{"API_KEY":"{{ api_key }}"}}`

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccEDACredentialTypeDataSourceByID(randomName, description),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.aap_eda_credential_type.test", "name", randomName),
					resource.TestCheckResourceAttr("data.aap_eda_credential_type.test", "description", description),
					resource.TestCheckResourceAttrSet("data.aap_eda_credential_type.test", "id"),
					resource.TestCheckResourceAttr("data.aap_eda_credential_type.test", "inputs", expectedInputs),
					resource.TestCheckResourceAttr("data.aap_eda_credential_type.test", "injectors", expectedInjectors),
				),
			},
		},
	})
}

func testAccEDACredentialTypeDataSourceByName(name string, description string) string {
	return fmt.Sprintf(`
resource "aap_eda_credential_type" "test" {
  name        = "%s"
  description = "%s"
  inputs      = jsonencode({
    fields = [
      {
        id    = "username"
        label = "Username"
        type  = "string"
      },
      {
        id     = "password"
        label  = "Password"
        type   = "string"
        secret = true
      }
    ]
  })
  injectors = jsonencode({
    env = {
      MY_USERNAME = "{{ username }}"
      MY_PASSWORD = "{{ password }}"
    }
  })
}

data "aap_eda_credential_type" "test" {
  name = aap_eda_credential_type.test.name
}
`, name, description)
}

func testAccEDACredentialTypeDataSourceByID(name string, description string) string {
	return fmt.Sprintf(`
resource "aap_eda_credential_type" "test" {
  name        = "%s"
  description = "%s"
  inputs      = jsonencode({
    fields = [
      {
        id     = "api_key"
        label  = "API Key"
        type   = "string"
        secret = true
      }
    ]
  })
  injectors = jsonencode({
    env = {
      API_KEY = "{{ api_key }}"
    }
  })
}

data "aap_eda_credential_type" "test" {
  id = aap_eda_credential_type.test.id
}
`, name, description)
}
