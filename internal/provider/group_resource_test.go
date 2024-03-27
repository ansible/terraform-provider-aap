package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/ansible/terraform-provider-aap/internal/provider/customtypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestGroupResourceSchema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schemaRequest := fwresource.SchemaRequest{}
	schemaResponse := &fwresource.SchemaResponse{}

	// Instantiate the GroupResource and call its Schema method
	NewGroupResource().Schema(ctx, schemaRequest, schemaResponse)

	if schemaResponse.Diagnostics.HasError() {
		t.Fatalf("Schema method diagnostics: %+v", schemaResponse.Diagnostics)
	}

	// Validate the schema
	diagnostics := schemaResponse.Schema.ValidateImplementation(ctx)

	if diagnostics.HasError() {
		t.Fatalf("Schema validation diagnostics: %+v", diagnostics)
	}
}

func TestGroupResourceCreateRequestBody(t *testing.T) {
	var testTable = []struct {
		name     string
		input    GroupResourceModel
		expected []byte
	}{
		{
			name: "test with unknown values",
			input: GroupResourceModel{
				Name:        types.StringValue("test group"),
				Description: types.StringUnknown(),
				URL:         types.StringUnknown(),
				Variables:   customtypes.NewAAPCustomStringUnknown(),
				InventoryId: types.Int64Value(0),
			},
			expected: []byte(`{"inventory":0,"name":"test group"}`),
		},
		{
			name: "test with null values",
			input: GroupResourceModel{
				Name:        types.StringValue("test group"),
				Description: types.StringNull(),
				URL:         types.StringNull(),
				Variables:   customtypes.NewAAPCustomStringNull(),
				InventoryId: types.Int64Value(0),
			},
			expected: []byte(`{"inventory":0,"name":"test group"}`),
		},
		{
			name: "test with some values",
			input: GroupResourceModel{
				InventoryId: types.Int64Value(1),
				Name:        types.StringValue("group1"),
				Description: types.StringNull(),
				URL:         types.StringValue("/api/v2/groups/1/"),
				Variables:   customtypes.NewAAPCustomStringValue("{\"foo\":\"bar\"}"),
			},
			expected: []byte(
				`{"inventory":1,"name":"group1","variables":"{\"foo\":\"bar\"}"}`,
			),
		},
		{
			name: "test with all values",
			input: GroupResourceModel{
				InventoryId: types.Int64Value(1),
				Name:        types.StringValue("group1"),
				Description: types.StringValue("A test group"),
				URL:         types.StringValue("/api/v2/groups/1/"),
				Variables:   customtypes.NewAAPCustomStringValue("{\"foo\":\"bar\"}"),
			},
			expected: []byte(
				`{"inventory":1,"name":"group1","description":"A test group","variables":"{\"foo\":\"bar\"}"}`,
			),
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			actual, diags := test.input.CreateRequestBody()
			if diags.HasError() {
				t.Fatal(diags.Errors())
			}
			if !bytes.Equal(test.expected, actual) {
				t.Errorf("Expected (%s) not equal to actual (%s)", test.expected, actual)
			}
		})
	}
}

func TestGroupResourceParseHttpResponse(t *testing.T) {
	jsonError := diag.Diagnostics{}
	jsonError.AddError("Error parsing JSON response from AAP", "invalid character 'N' looking for beginning of value")

	var testTable = []struct {
		name     string
		input    []byte
		expected GroupResourceModel
		errors   diag.Diagnostics
	}{
		{
			name:     "test with JSON error",
			input:    []byte("Not valid JSON"),
			expected: GroupResourceModel{},
			errors:   jsonError,
		},
		{
			name:  "test with missing values",
			input: []byte(`{"inventory":1, "id": 0, "name": "group1", "url": "/api/v2/groups/1/", "description": ""}`),
			expected: GroupResourceModel{
				InventoryId: types.Int64Value(1),
				Id:          types.Int64Value(0),
				Name:        types.StringValue("group1"),
				URL:         types.StringValue("/api/v2/groups/1/"),
				Description: types.StringNull(),
			},
			errors: diag.Diagnostics{},
		},
		{
			name: "test with all values",
			input: []byte(`{"inventory":1,"description":"A basic test group","name":"group1","url":"/api/v2/groups/1/",` +
				`"variables":"{\"foo\":\"bar\",\"nested\":{\"foobar\":\"baz\"}}"}`),
			expected: GroupResourceModel{
				InventoryId: types.Int64Value(1),
				Id:          types.Int64Value(0),
				Name:        types.StringValue("group1"),
				URL:         types.StringValue("/api/v2/groups/1/"),
				Description: types.StringValue("A basic test group"),
				Variables:   customtypes.NewAAPCustomStringValue("{\"foo\":\"bar\",\"nested\":{\"foobar\":\"baz\"}}"),
			},
			errors: diag.Diagnostics{},
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			resource := GroupResourceModel{}
			diags := resource.ParseHttpResponse(test.input)
			if !test.errors.Equal(diags) {
				t.Errorf("Expected error diagnostics (%s), actual was (%s)", test.errors, diags)
			}
			if !reflect.DeepEqual(test.expected, resource) {
				t.Errorf("Expected (%s) not equal to actual (%s)", test.expected, resource)
			}
		})
	}
}

// Acceptance tests

func TestAccGroupResource(t *testing.T) {
	var groupApiModel GroupAPIModel
	var description = "A test group"
	var variables = "{\"foo\": \"bar\"}"
	inventoryName := acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)
	groupName := acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)
	updatedName := "updated" + groupName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Invalid variables testing
			{
				Config:      testAccGroupResourceBadVariables(inventoryName, updatedName),
				ExpectError: regexp.MustCompile("Input type `str` is not a dictionary"),
			},
			// Create and Read testing
			{
				Config: testAccGroupResourceMinimal(inventoryName, groupName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckGroupResourceExists("aap_group.test", &groupApiModel),
					testAccCheckGroupResourceValues(&groupApiModel, groupName, "", ""),
					resource.TestCheckResourceAttr("aap_group.test", "name", groupName),
					resource.TestCheckResourceAttrPair("aap_group.test", "inventory_id", "aap_inventory.test", "id"),
					resource.TestMatchResourceAttr("aap_group.test", "url", regexp.MustCompile("^/api/v2/groups/[0-9]*/$")),
				),
			},
			{
				Config: testAccGroupResourceComplete(inventoryName, updatedName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckGroupResourceExists("aap_group.test", &groupApiModel),
					testAccCheckGroupResourceValues(&groupApiModel, updatedName, description, variables),
					resource.TestCheckResourceAttr("aap_group.test", "name", updatedName),
					resource.TestCheckResourceAttrPair("aap_group.test", "inventory_id", "aap_inventory.test", "id"),
					resource.TestCheckResourceAttr("aap_group.test", "description", description),
					resource.TestCheckResourceAttr("aap_group.test", "variables", variables),
					resource.TestMatchResourceAttr("aap_group.test", "url", regexp.MustCompile("^/api/v2/groups/[0-9]*/$")),
				),
			},
		},
		CheckDestroy: testAccCheckGroupResourceDestroy,
	})
}

func testAccGroupResourceMinimal(inventoryName, groupName string) string {
	return fmt.Sprintf(`
resource "aap_inventory" "test" {
  name = "%s"
}

resource "aap_group" "test" {
  name = "%s"
  inventory_id = aap_inventory.test.id
}`, inventoryName, groupName)
}

func testAccGroupResourceComplete(inventoryName, groupName string) string {
	return fmt.Sprintf(`
resource "aap_inventory" "test" {
  name = "%s"
}

resource "aap_group" "test" {
  name = "%s"
  inventory_id = aap_inventory.test.id
  description = "A test group"
  variables = "{\"foo\": \"bar\"}"
}`, inventoryName, groupName)
}

// testAccGroupResourceBadVariables returns a configuration for an AAP group with the provided name and invalid variables.
func testAccGroupResourceBadVariables(inventoryName, groupName string) string {
	return fmt.Sprintf(`
resource "aap_inventory" "test" {
  name = "%s"
}

resource "aap_group" "test" {
  name = "%s"
  inventory_id = aap_inventory.test.id
  variables = "Not valid JSON"
}`, inventoryName, groupName)
}

// testAccCheckGroupResourceExists queries the AAP API and retrieves the matching group.
func testAccCheckGroupResourceExists(name string, groupApiModel *GroupAPIModel) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		groupResource, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("group (%s) not found in state", name)
		}

		groupResponseBody, err := testGetResource(groupResource.Primary.Attributes["url"])
		if err != nil {
			return err
		}

		err = json.Unmarshal(groupResponseBody, &groupApiModel)
		if err != nil {
			return err
		}

		if groupApiModel.Id == 0 {
			return fmt.Errorf("group (%s) not found in AAP", groupResource.Primary.ID)
		}

		return nil
	}
}

func testAccCheckGroupResourceValues(groupApiModel *GroupAPIModel, name string, description string, variables string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if groupApiModel.URL == "" {
			return fmt.Errorf("bad group URL in AAP, expected a URL path, got: %s", groupApiModel.URL)
		}
		if groupApiModel.Name != name {
			return fmt.Errorf("bad group name in AAP, expected \"%s\", got: %s", name, groupApiModel.Name)
		}
		if groupApiModel.Description != description {
			return fmt.Errorf("bad group description in AAP, expected \"%s\", got: %s", description, groupApiModel.Description)
		}
		if groupApiModel.Variables != variables {
			return fmt.Errorf("bad group variables in AAP, expected \"%s\", got: %s", variables, groupApiModel.Variables)
		}

		return nil
	}
}

// testAccCheckGroupResourceDestroy verifies the group has been destroyed.
func testAccCheckGroupResourceDestroy(s *terraform.State) error {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "group" {
			continue
		}

		_, err := testGetResource(rs.Primary.Attributes["url"])
		if err == nil {
			return fmt.Errorf("group (%s) still exists.", rs.Primary.Attributes["id"])
		}

		if !strings.Contains(err.Error(), "404") {
			return err
		}
	}

	return nil
}
