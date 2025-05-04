package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

func TestInventoryResourceSchema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schemaRequest := fwresource.SchemaRequest{}
	schemaResponse := &fwresource.SchemaResponse{}

	// Instantiate the InventoryResource and call its Schema method
	NewInventoryResource().Schema(ctx, schemaRequest, schemaResponse)

	if schemaResponse.Diagnostics.HasError() {
		t.Fatalf("Schema method diagnostics: %+v", schemaResponse.Diagnostics)
	}

	// Validate the schema
	diagnostics := schemaResponse.Schema.ValidateImplementation(ctx)

	if diagnostics.HasError() {
		t.Fatalf("Schema validation diagnostics: %+v", diagnostics)
	}
}

func TestInventoryResourceGenerateRequestBody(t *testing.T) {
	var testTable = []struct {
		name     string
		input    InventoryResourceModel
		expected []byte
	}{
		{
			name: "unknown values",
			input: InventoryResourceModel{
				Id:           types.Int64Unknown(),
				Organization: types.Int64Unknown(),
				Url:          types.StringUnknown(),
				Name:         types.StringValue("test inventory"),
				Description:  types.StringUnknown(),
				Variables:    customtypes.NewAAPCustomStringUnknown(),
			},
			expected: []byte(`{"organization":1,"name":"test inventory"}`),
		},
		{
			name: "null values",
			input: InventoryResourceModel{
				Id:           types.Int64Null(),
				Organization: types.Int64Null(),
				Url:          types.StringNull(),
				Name:         types.StringValue("test inventory"),
				Description:  types.StringNull(),
				Variables:    customtypes.NewAAPCustomStringNull(),
			},
			expected: []byte(`{"organization":1,"name":"test inventory"}`),
		},
		{
			name: "provided values",
			input: InventoryResourceModel{
				Id:           types.Int64Value(1),
				Organization: types.Int64Value(2),
				Url:          types.StringValue("/inventories/1/"),
				Name:         types.StringValue("test inventory"),
				Description:  types.StringValue("A test inventory for testing"),
				Variables:    customtypes.NewAAPCustomStringValue("{\"foo\": \"bar\", \"nested\": {\"foobar\": \"baz\"}}"),
			},
			expected: []byte(
				`{"organization":2,"name":"test inventory","description":"A test inventory for testing",` +
					`"variables":"{\"foo\": \"bar\", \"nested\": {\"foobar\": \"baz\"}}"}`,
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

func TestInventoryResourceParseHttpResponse(t *testing.T) {
	jsonError := diag.Diagnostics{}
	jsonError.AddError("Error parsing JSON response from AAP", "invalid character 'N' looking for beginning of value")

	var testTable = []struct {
		name     string
		input    []byte
		expected InventoryResourceModel
		errors   diag.Diagnostics
	}{
		{
			name:     "JSON error",
			input:    []byte("Not valid JSON"),
			expected: InventoryResourceModel{},
			errors:   jsonError,
		},
		{
			name:  "missing values",
			input: []byte(`{"id":1,"type":"inventory","name":"test inventory","organization":2,"url":"/inventories/1/"}`),
			expected: InventoryResourceModel{
				Id:           types.Int64Value(1),
				Organization: types.Int64Value(2),
				Url:          types.StringValue("/inventories/1/"),
				Name:         types.StringValue("test inventory"),
				Description:  types.StringNull(),
				Variables:    customtypes.NewAAPCustomStringNull(),
			},
			errors: diag.Diagnostics{},
		},
		{
			name: "all values",
			input: []byte(
				`{"description":"A test inventory for testing","id":1,"name":"test inventory","organization":2,` +
					`"type":"inventory","url":"/inventories/1/","variables":"{\"foo\":\"bar\",\"nested\":{\"foobar\":\"baz\"}}"}`,
			),
			expected: InventoryResourceModel{
				Id:           types.Int64Value(1),
				Organization: types.Int64Value(2),
				Url:          types.StringValue("/inventories/1/"),
				Name:         types.StringValue("test inventory"),
				Description:  types.StringValue("A test inventory for testing"),
				Variables:    customtypes.NewAAPCustomStringValue("{\"foo\":\"bar\",\"nested\":{\"foobar\":\"baz\"}}"),
			},
			errors: diag.Diagnostics{},
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			resource := InventoryResourceModel{}
			diags := resource.parseHTTPResponse(test.input)
			if !test.errors.Equal(diags) {
				t.Errorf("Expected error diagnostics (%s), actual was (%s)", test.errors, diags)
			}
			if test.expected != resource {
				t.Errorf("Expected (%s) not equal to actual (%s)", test.expected, resource)
			}
		})
	}
}

func TestAccInventoryResource(t *testing.T) {
	var inventory InventoryAPIModel
	randomName := acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)
	updatedName := "updated " + randomName
	updatedDescription := "A test inventory"
	updatedVariables := "{\"foo\": \"bar\"}"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Invalid variables testing
			{
				Config:      testAccInventoryResourceBadVariables(updatedName),
				ExpectError: regexp.MustCompile("Input type `str` is not a dictionary"),
			},
			// Create and Read testing
			{
				Config: testAccInventoryResourceMinimal(randomName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckInventoryResourceExists("aap_inventory.test", &inventory),
					testAccCheckInventoryResourceValues(&inventory, randomName, "", ""),
					resource.TestCheckResourceAttr("aap_inventory.test", "name", randomName),
					resource.TestCheckResourceAttr("aap_inventory.test", "organization", "1"),
					resource.TestCheckResourceAttrSet("aap_inventory.test", "id"),
					resource.TestCheckResourceAttrSet("aap_inventory.test", "url"),
				),
			},
			// Update and Read testing
			{
				Config: testAccInventoryResourceComplete(updatedName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckInventoryResourceExists("aap_inventory.test", &inventory),
					testAccCheckInventoryResourceValues(&inventory, updatedName, updatedDescription, updatedVariables),
					resource.TestCheckResourceAttr("aap_inventory.test", "name", updatedName),
					resource.TestCheckResourceAttr("aap_inventory.test", "organization", "1"),
					resource.TestCheckResourceAttr("aap_inventory.test", "description", updatedDescription),
					resource.TestCheckResourceAttr("aap_inventory.test", "variables", updatedVariables),
					resource.TestCheckResourceAttrSet("aap_inventory.test", "id"),
					resource.TestCheckResourceAttrSet("aap_inventory.test", "url"),
				),
			},
		},
		CheckDestroy: testAccCheckInventoryResourceDestroy,
	})
}

// testAccInventoryResourceMinimal returns a configuration for an AAP Inventory with the provided name only.
func testAccInventoryResourceMinimal(name string) string {
	return fmt.Sprintf(`
resource "aap_inventory" "test" {
  name = "%s"
}`, name)
}

// testAccInventoryResourceComplete returns a configuration for an AAP Inventory with the provided name and all options.
func testAccInventoryResourceComplete(name string) string {
	return fmt.Sprintf(`
resource "aap_inventory" "test" {
  name = "%s"
  description = "A test inventory"
  variables = "{\"foo\": \"bar\"}"
}`, name)
}

// testAccInventoryResourceBadVariables returns a configuration for an AAP Inventory with the provided name and invalid variables.
func testAccInventoryResourceBadVariables(name string) string {
	return fmt.Sprintf(`
resource "aap_inventory" "test" {
  name = "%s"
  variables = "Not valid JSON"
}`, name)
}

// testAccCheckInventoryResourceExists queries the AAP API and retrieves the matching inventory.
func testAccCheckInventoryResourceExists(name string, inventory *InventoryAPIModel) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		inventoryResource, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("inventory (%s) not found in state", name)
		}

		inventoryResponseBody, err := testGetResource(inventoryResource.Primary.Attributes["url"])
		if err != nil {
			return err
		}

		err = json.Unmarshal(inventoryResponseBody, &inventory)
		if err != nil {
			return err
		}

		if inventory.Id == 0 {
			return fmt.Errorf("inventory (%s) not found in AAP", inventoryResource.Primary.ID)
		}

		return nil
	}
}

// testAccCheckInventoryResourcesValues verifies that the provided inventory retrieved from AAP contains the expected values.
func testAccCheckInventoryResourceValues(inventory *InventoryAPIModel, name string, description string, variables string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		if inventory.Id == 0 {
			return fmt.Errorf("bad inventory ID in AAP, expected a positive int64, got: %dv", inventory.Id)
		}
		if inventory.Organization == 0 {
			return fmt.Errorf("bad inventory organization in AAP, expected a positive int64, got: %d", inventory.Organization)
		}
		if inventory.Url == "" {
			return fmt.Errorf("bad inventory URL in AAP, expected a URL path, got: %s", inventory.Url)
		}
		if inventory.Name != name {
			return fmt.Errorf("bad inventory name in AAP, expected \"%s\", got: %s", name, inventory.Name)
		}
		if inventory.Description != description {
			return fmt.Errorf("bad inventory description in AAP, expected \"%s\", got: %s", description, inventory.Description)
		}
		if inventory.Variables != variables {
			return fmt.Errorf("bad inventory variables in AAP, expected \"%s\", got: %s", variables, inventory.Variables)
		}
		return nil
	}
}

// testAccCheckInventoryDestroy verifies the inventory has been destroyed.
func testAccCheckInventoryResourceDestroy(s *terraform.State) error {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "inventory" {
			continue
		}

		_, err := testGetResource(rs.Primary.Attributes["url"])
		if err == nil {
			return fmt.Errorf("inventory (%s) still exists.", rs.Primary.Attributes["id"])
		}

		if !strings.Contains(err.Error(), "404") {
			return err
		}
	}

	return nil
}
