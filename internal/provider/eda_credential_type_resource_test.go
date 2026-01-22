package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	tftypes "github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const resourceNameEDACredentialType = "aap_eda_credential_type.test"

func TestEDACredentialTypeResourceSchema(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	schemaRequest := fwresource.SchemaRequest{}
	schemaResponse := &fwresource.SchemaResponse{}

	NewEDACredentialTypeResource().Schema(ctx, schemaRequest, schemaResponse)

	if schemaResponse.Diagnostics.HasError() {
		t.Fatalf("Schema method diagnostics: %+v", schemaResponse.Diagnostics)
	}

	diagnostics := schemaResponse.Schema.ValidateImplementation(ctx)

	if diagnostics.HasError() {
		t.Fatalf("Schema validation diagnostics: %+v", diagnostics)
	}
}

func TestEDACredentialTypeResourceGenerateRequestBody(t *testing.T) {
	var testTable = []struct {
		name     string
		input    EDACredentialTypeResourceModel
		expected []byte
	}{
		{
			name: "unknown values",
			input: EDACredentialTypeResourceModel{
				ID:          tftypes.Int64Unknown(),
				Name:        tftypes.StringUnknown(),
				Description: tftypes.StringUnknown(),
				Inputs:      tftypes.StringUnknown(),
				Injectors:   tftypes.StringUnknown(),
			},
			expected: []byte(`{"name":""}`),
		},
		{
			name: "null values",
			input: EDACredentialTypeResourceModel{
				ID:          tftypes.Int64Null(),
				Name:        tftypes.StringNull(),
				Description: tftypes.StringNull(),
				Inputs:      tftypes.StringNull(),
				Injectors:   tftypes.StringNull(),
			},
			expected: []byte(`{"name":""}`),
		},
		{
			name: "provided values",
			input: EDACredentialTypeResourceModel{
				ID:          tftypes.Int64Value(1),
				Name:        tftypes.StringValue("test credential type"),
				Description: tftypes.StringValue("A test credential type"),
				Inputs:      tftypes.StringValue(`{"fields":[{"id":"username","label":"Username","type":"string"}]}`),
				Injectors:   tftypes.StringValue(`{"env":{"MY_VAR":"{{ username }}"}}`),
			},
			expected: []byte(
				`{"name":"test credential type","description":"A test credential type",` +
					`"inputs":{"fields":[{"id":"username","label":"Username","type":"string"}]},` +
					`"injectors":{"env":{"MY_VAR":"{{ username }}"}}}`,
			),
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			actual, diags := test.input.generateRequestBody()
			if diags.HasError() {
				t.Fatal(diags.Errors())
			}
			if !bytes.Equal(test.expected, actual) {
				t.Errorf("Expected (%s) not equal to actual (%s)", test.expected, actual)
			}
		})
	}
}

func TestEDACredentialTypeResourceParseHTTPResponse(t *testing.T) {
	jsonError := diag.Diagnostics{}
	jsonError.AddError("Error parsing JSON response from EDA", "invalid character 'N' looking for beginning of value")

	var testTable = []struct {
		name     string
		input    []byte
		expected EDACredentialTypeResourceModel
		errors   diag.Diagnostics
	}{
		{
			name:     "JSON error",
			input:    []byte("Not valid JSON"),
			expected: EDACredentialTypeResourceModel{},
			errors:   jsonError,
		},
		{
			name:  "missing values",
			input: []byte(`{"id":1,"name":"test credential type","url":"/api/eda/v1/credential-types/1/"}`),
			expected: EDACredentialTypeResourceModel{
				ID:          tftypes.Int64Value(1),
				Name:        tftypes.StringValue("test credential type"),
				Description: tftypes.StringNull(),
				Inputs:      tftypes.StringNull(),
				Injectors:   tftypes.StringNull(),
			},
			errors: diag.Diagnostics{},
		},
		{
			name: "all values",
			input: []byte(
				`{"id":1,"name":"test credential type","description":"A test credential type",` +
					`"url":"/api/eda/v1/credential-types/1/",` +
					`"inputs":{"fields":[{"id":"username","label":"Username","type":"string"}]},` +
					`"injectors":{"env":{"MY_VAR":"{{ username }}"}}}`,
			),
			expected: EDACredentialTypeResourceModel{
				ID:          tftypes.Int64Value(1),
				Name:        tftypes.StringValue("test credential type"),
				Description: tftypes.StringValue("A test credential type"),
				Inputs:      tftypes.StringValue(`{"fields":[{"id":"username","label":"Username","type":"string"}]}`),
				Injectors:   tftypes.StringValue(`{"env":{"MY_VAR":"{{ username }}"}}`),
			},
			errors: diag.Diagnostics{},
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			resource := EDACredentialTypeResourceModel{}
			diags := resource.parseHTTPResponse(test.input)
			if !test.errors.Equal(diags) {
				t.Errorf("Expected error diagnostics (%s), actual was (%s)", test.errors, diags)
			}
			if test.expected != resource {
				t.Errorf("Expected (%+v) not equal to actual (%+v)", test.expected, resource)
			}
		})
	}
}

func TestAccEDACredentialTypeResource(t *testing.T) {
	var credentialType EDACredentialTypeAPIModel
	randomName := acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)
	updatedName := "updated " + randomName
	updatedDescription := "An updated test credential type"

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			skipTestWithoutEDAPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccEDACredentialTypeResourceMinimal(randomName),
				Check:  checkBasicEDACredentialTypeAttributes(t, resourceNameEDACredentialType, credentialType, randomName, "", "", ""),
			},
			{
				Config: testAccEDACredentialTypeResourceComplete(updatedName, updatedDescription),
				Check:  checkBasicEDACredentialTypeAttributesComplete(t, resourceNameEDACredentialType, credentialType, updatedName, updatedDescription),
			},
		},
		CheckDestroy: testAccCheckEDACredentialTypeResourceDestroy,
	})
}

func testAccEDACredentialTypeResourceMinimal(name string) string {
	return fmt.Sprintf(`
resource "aap_eda_credential_type" "test" {
  name = "%s"
}`, name)
}

func testAccEDACredentialTypeResourceComplete(name string, description string) string {
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
`, name, description)
}

func checkBasicEDACredentialTypeAttributes(t *testing.T, name string, credentialType EDACredentialTypeAPIModel, expectedName string, expectedDescription string, expectedInputs string, expectedInjectors string) resource.TestCheckFunc {
	checks := []resource.TestCheckFunc{
		testAccCheckEDACredentialTypeResourceExists(name, &credentialType),
		testAccCheckEDACredentialTypeResourceValues(&credentialType, expectedName, expectedDescription, expectedInputs, expectedInjectors),
		resource.TestCheckResourceAttr(name, "name", expectedName),
		resource.TestCheckResourceAttrSet(name, "id"),
	}

	if expectedDescription != "" {
		checks = append(checks, resource.TestCheckResourceAttr(name, "description", expectedDescription))
	}
	if expectedInputs != "" {
		checks = append(checks, resource.TestCheckResourceAttr(name, "inputs", expectedInputs))
	}
	if expectedInjectors != "" {
		checks = append(checks, resource.TestCheckResourceAttr(name, "injectors", expectedInjectors))
	}

	return resource.ComposeAggregateTestCheckFunc(checks...)
}

func checkBasicEDACredentialTypeAttributesComplete(t *testing.T, name string, credentialType EDACredentialTypeAPIModel, expectedName string, expectedDescription string) resource.TestCheckFunc {
	expectedInputs := `{"fields":[{"id":"username","label":"Username","type":"string"},{"id":"password","label":"Password","secret":true,"type":"string"}]}`
	expectedInjectors := `{"env":{"MY_PASSWORD":"{{ password }}","MY_USERNAME":"{{ username }}"}}`

	return resource.ComposeAggregateTestCheckFunc(
		testAccCheckEDACredentialTypeResourceExists(name, &credentialType),
		testAccCheckEDACredentialTypeResourceValues(&credentialType, expectedName, expectedDescription, expectedInputs, expectedInjectors),
		resource.TestCheckResourceAttr(name, "name", expectedName),
		resource.TestCheckResourceAttr(name, "description", expectedDescription),
		resource.TestCheckResourceAttrSet(name, "id"),
		resource.TestCheckResourceAttrSet(name, "inputs"),
		resource.TestCheckResourceAttrSet(name, "injectors"),
	)
}

func testAccCheckEDACredentialTypeResourceExists(name string, credentialType *EDACredentialTypeAPIModel) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		credentialTypeResource, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("credential type (%s) not found in state", name)
		}

		id := credentialTypeResource.Primary.Attributes["id"]
		if id == "" {
			return fmt.Errorf("credential type ID is empty in state")
		}

		url := fmt.Sprintf("/api/eda/v1/credential-types/%s/", id)
		credentialTypeResponseBody, err := testGetResource(url)
		if err != nil {
			return fmt.Errorf("error getting resource from URL %s: %w", url, err)
		}

		err = json.Unmarshal(credentialTypeResponseBody, &credentialType)
		if err != nil {
			return err
		}

		if credentialType.ID == 0 {
			return fmt.Errorf("credential type (%s) not found in EDA", credentialTypeResource.Primary.ID)
		}

		return nil
	}
}

func jsonEqual(s1, s2 string) bool {
	var o1, o2 interface{}

	if err := json.Unmarshal([]byte(s1), &o1); err != nil {
		return false
	}
	if err := json.Unmarshal([]byte(s2), &o2); err != nil {
		return false
	}

	b1, err := json.Marshal(o1)
	if err != nil {
		return false
	}
	b2, err := json.Marshal(o2)
	if err != nil {
		return false
	}

	return bytes.Equal(b1, b2)
}

func testAccCheckEDACredentialTypeResourceValues(credentialType *EDACredentialTypeAPIModel, name string, description string, inputs string, injectors string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		if credentialType.ID == 0 {
			return fmt.Errorf("bad credential type ID in EDA, expected a positive int64, got: %d", credentialType.ID)
		}
		if credentialType.Name != name {
			return fmt.Errorf("bad credential type name in EDA, expected \"%s\", got: %s", name, credentialType.Name)
		}
		if credentialType.Description != description {
			return fmt.Errorf("bad credential type description in EDA, expected \"%s\", got: %s", description, credentialType.Description)
		}
		expectedInputs := strings.TrimSpace(inputs)
		if expectedInputs == "" {
			expectedInputs = JSONEmptyObject
		}
		actualInputs := strings.TrimSpace(string(credentialType.Inputs))

		if !jsonEqual(expectedInputs, actualInputs) {
			return fmt.Errorf("bad credential type inputs in EDA, expected \"%s\", got: %s", expectedInputs, actualInputs)
		}

		expectedInjectors := strings.TrimSpace(injectors)
		if expectedInjectors == "" {
			expectedInjectors = JSONEmptyObject
		}
		actualInjectors := strings.TrimSpace(string(credentialType.Injectors))

		if !jsonEqual(expectedInjectors, actualInjectors) {
			return fmt.Errorf("bad credential type injectors in EDA, expected \"%s\", got: %s", expectedInjectors, actualInjectors)
		}
		return nil
	}
}

func testAccCheckEDACredentialTypeResourceDestroy(s *terraform.State) error {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "aap_eda_credential_type" {
			continue
		}

		id := rs.Primary.Attributes["id"]
		url := fmt.Sprintf("/api/eda/v1/credential-types/%s/", id)
		_, err := testGetResource(url)
		if err == nil {
			return fmt.Errorf("credential type (%s) still exists", rs.Primary.Attributes["id"])
		}

		if !strings.Contains(err.Error(), "404") {
			return err
		}
	}

	return nil
}
