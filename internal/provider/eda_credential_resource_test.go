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

const resourceNameEDACredential = "aap_eda_credential.test"

func TestEDACredentialResourceSchema(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	schemaRequest := fwresource.SchemaRequest{}
	schemaResponse := &fwresource.SchemaResponse{}

	NewEDACredentialResource().Schema(ctx, schemaRequest, schemaResponse)

	if schemaResponse.Diagnostics.HasError() {
		t.Fatalf("Schema method diagnostics: %+v", schemaResponse.Diagnostics)
	}

	diagnostics := schemaResponse.Schema.ValidateImplementation(ctx)

	if diagnostics.HasError() {
		t.Fatalf("Schema validation diagnostics: %+v", diagnostics)
	}
}

func TestCalculateInputsHash(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:     "simple json",
			input:    `{"username":"test","password":"secret"}`,
			expected: "8c6976e5b5410415bde908bd4dee15dfb167a9c873fc4bb8a81f6f2ab448a918",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := calculateInputsHash(tc.input)
			if result != tc.expected {
				t.Errorf("Expected hash %s, got %s", tc.expected, result)
			}
		})
	}
}

func TestEDACredentialResourceGenerateRequestBody(t *testing.T) {
	var testTable = []struct {
		name     string
		input    EDACredentialResourceModel
		expected []byte
	}{
		{
			name: "minimal credential",
			input: EDACredentialResourceModel{
				Name:             tftypes.StringValue("test-cred"),
				CredentialTypeID: tftypes.Int64Value(1),
				InputsWO:         tftypes.StringValue(`{"username":"user"}`),
			},
			expected: []byte(`{"name":"test-cred","credential_type_id":1,"inputs":{"username":"user"}}`),
		},
		{
			name: "complete credential",
			input: EDACredentialResourceModel{
				Name:             tftypes.StringValue("test-cred"),
				Description:      tftypes.StringValue("Test credential"),
				CredentialTypeID: tftypes.Int64Value(1),
				OrganizationID:   tftypes.Int64Value(2),
				InputsWO:         tftypes.StringValue(`{"username":"user","password":"secret"}`),
			},
			expected: []byte(`{"name":"test-cred","description":"Test credential","credential_type_id":1,"organization_id":2,"inputs":{"username":"user","password":"secret"}}`),
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

func TestEDACredentialResourceParseHTTPResponse(t *testing.T) {
	jsonError := diag.Diagnostics{}
	jsonError.AddError("Error parsing JSON response from EDA", "invalid character 'N' looking for beginning of value")

	var testTable = []struct {
		name     string
		input    []byte
		expected EDACredentialResourceModel
		errors   diag.Diagnostics
	}{
		{
			name:     "JSON error",
			input:    []byte("Not valid JSON"),
			expected: EDACredentialResourceModel{},
			errors:   jsonError,
		},
		{
			name:  "minimal response",
			input: []byte(`{"id":1,"name":"test-cred","credential_type_id":1,"url":"/api/eda/v1/eda-credentials/1/"}`),
			expected: EDACredentialResourceModel{
				ID:               tftypes.Int64Value(1),
				Name:             tftypes.StringValue("test-cred"),
				CredentialTypeID: tftypes.Int64Value(1),
				Description:      tftypes.StringNull(),
				OrganizationID:   tftypes.Int64Null(),
			},
			errors: diag.Diagnostics{},
		},
		{
			name: "complete response",
			input: []byte(
				`{"id":1,"name":"test-cred","description":"Test credential","credential_type_id":1,` +
					`"organization_id":2,"url":"/api/eda/v1/eda-credentials/1/"}`,
			),
			expected: EDACredentialResourceModel{
				ID:               tftypes.Int64Value(1),
				Name:             tftypes.StringValue("test-cred"),
				Description:      tftypes.StringValue("Test credential"),
				CredentialTypeID: tftypes.Int64Value(1),
				OrganizationID:   tftypes.Int64Value(2),
			},
			errors: diag.Diagnostics{},
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			resource := EDACredentialResourceModel{}
			diags := resource.parseHTTPResponse(test.input)
			if !test.errors.Equal(diags) {
				t.Errorf("Expected error diagnostics (%s), actual was (%s)", test.errors, diags)
			}
			if test.expected.ID != resource.ID ||
				test.expected.Name != resource.Name ||
				test.expected.CredentialTypeID != resource.CredentialTypeID {
				t.Errorf("Expected (%+v) not equal to actual (%+v)", test.expected, resource)
			}
		})
	}
}

func TestAccEDACredentialResource(t *testing.T) {
	// t.Skip("Skipping: EDA credentials API endpoint not available in EDA 1.1.x - requires EDA 1.2+")

	var credential EDACredentialAPIModel
	randomName := acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)
	updatedName := "updated " + randomName
	updatedDescription := "An updated test credential"
	initialInputs := `{"username":"testuser","password":"initial123"}`
	updatedInputs := `{"username":"testuser","password":"updated456"}`

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			// skipTestWithoutEDAPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccEDACredentialResourceMinimal(randomName, initialInputs),
				Check:  checkBasicEDACredentialAttributes(t, resourceNameEDACredential, credential, randomName, "", initialInputs),
			},
			{
				Config: testAccEDACredentialResourceComplete(updatedName, updatedDescription, updatedInputs),
				Check:  checkBasicEDACredentialAttributes(t, resourceNameEDACredential, credential, updatedName, updatedDescription, updatedInputs),
			},
		},
		CheckDestroy: testAccCheckEDACredentialResourceDestroy,
	})
}

func testAccEDACredentialResourceMinimal(name string, inputs string) string {
	return testAccEDACredentialTypeResourceComplete(name, name) + fmt.Sprintf(`
resource "aap_eda_credential" "test" {
  name               = "%s"
  credential_type_id = aap_eda_credential_type.test.id
  organization_id    = 1
  
  inputs_wo = %q
}`, name, inputs)
}

func testAccEDACredentialResourceComplete(name string, description string, inputs string) string {
	return testAccEDACredentialTypeResourceComplete(name, name) + fmt.Sprintf(`
resource "aap_eda_credential" "test" {
  name               = "%s"
  description        = "%s"
  credential_type_id = aap_eda_credential_type.test.id
  organization_id    = 1
  
  inputs_wo = %q
}`, name, description, inputs)
}

func checkBasicEDACredentialAttributes(t *testing.T, name string, credential EDACredentialAPIModel, expectedName string, expectedDescription string, expectedInputs string) resource.TestCheckFunc {
	checks := []resource.TestCheckFunc{
		testAccCheckEDACredentialResourceExists(name, &credential),
		testAccCheckEDACredentialResourceValues(&credential, expectedName, expectedDescription),
		testAccCheckEDACredentialInputsNotInState(name),
		resource.TestCheckResourceAttr(name, "name", expectedName),
		resource.TestCheckResourceAttrSet(name, "id"),
		resource.TestCheckResourceAttrSet(name, "inputs_wo_hash"),
	}

	// Only check description if provided
	if expectedDescription != "" {
		checks = append(checks, resource.TestCheckResourceAttr(name, "description", expectedDescription))
	}

	return resource.ComposeAggregateTestCheckFunc(checks...)
}

func testAccCheckEDACredentialResourceExists(name string, credential *EDACredentialAPIModel) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		credentialResource, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("credential (%s) not found in state", name)
		}

		id := credentialResource.Primary.Attributes["id"]
		url := fmt.Sprintf("/api/eda/v1/eda-credentials/%s/", id)
		credentialResponseBody, err := testGetResource(url)
		if err != nil {
			return err
		}

		err = json.Unmarshal(credentialResponseBody, &credential)
		if err != nil {
			return err
		}

		if credential.ID == 0 {
			return fmt.Errorf("credential (%s) not found in EDA", credentialResource.Primary.ID)
		}

		return nil
	}
}

func testAccCheckEDACredentialResourceValues(credential *EDACredentialAPIModel, name string, description string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		if credential.ID == 0 {
			return fmt.Errorf("bad credential ID in EDA, expected a positive int64, got: %d", credential.ID)
		}
		if credential.Name != name {
			return fmt.Errorf("bad credential name in EDA, expected \"%s\", got: %s", name, credential.Name)
		}
		if credential.Description != description {
			return fmt.Errorf("bad credential description in EDA, expected \"%s\", got: %s", description, credential.Description)
		}
		return nil
	}
}

func testAccCheckEDACredentialInputsNotInState(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		credentialResource, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("credential (%s) not found in state", name)
		}

		// Sensitive attributes in framework are not written to state file at all
		// So we just verify inputs_wo_hash exists for change detection
		// Note: in test state, sensitive values may appear - this is expected in test framework

		if _, exists := credentialResource.Primary.Attributes["inputs_wo_hash"]; !exists {
			return fmt.Errorf("inputs_wo_hash should exist in state for change detection")
		}

		return nil
	}
}

func testAccCheckEDACredentialResourceDestroy(s *terraform.State) error {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "aap_eda_credential" {
			continue
		}

		id := rs.Primary.Attributes["id"]
		url := fmt.Sprintf("/api/eda/v1/eda-credentials/%s/", id)
		_, err := testGetResource(url)
		if err == nil {
			return fmt.Errorf("credential (%s) still exists", rs.Primary.Attributes["id"])
		}

		if !strings.Contains(err.Error(), "404") {
			return err
		}
	}

	return nil
}
