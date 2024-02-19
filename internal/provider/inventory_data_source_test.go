package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	fwresource "github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestInventoryDataSourceSchema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schemaRequest := fwresource.SchemaRequest{}
	schemaResponse := &fwresource.SchemaResponse{}

	// Instantiate the InventoryDataSource and call its Schema method
	NewInventoryDataSource().Schema(ctx, schemaRequest, schemaResponse)

	if schemaResponse.Diagnostics.HasError() {
		t.Fatalf("Schema method diagnostics: %+v", schemaResponse.Diagnostics)
	}

	// Validate the schema
	diagnostics := schemaResponse.Schema.ValidateImplementation(ctx)

	if diagnostics.HasError() {
		t.Fatalf("Schema validation diagnostics: %+v", diagnostics)
	}
}

func TestInventoryDataSourceParseHttpResponse(t *testing.T) {
	jsonError := diag.Diagnostics{}
	jsonError.AddError("Error parsing JSON response from AAP", "invalid character 'N' looking for beginning of value")

	var testTable = []struct {
		name     string
		input    []byte
		expected InventoryDataSourceModel
		errors   diag.Diagnostics
	}{
		{
			name:     "JSON error",
			input:    []byte("Not valid JSON"),
			expected: InventoryDataSourceModel{},
			errors:   jsonError,
		},
		{
			name:  "missing values",
			input: []byte(`{"id":1,"organization":2,"url":"/inventories/1/"}`),
			expected: InventoryDataSourceModel{
				Id:           types.Int64Value(1),
				Organization: types.Int64Value(2),
				Url:          types.StringValue("/inventories/1/"),
				Name:         types.StringNull(),
				Description:  types.StringNull(),
				Variables:    jsontypes.NewNormalizedNull(),
			},
			errors: diag.Diagnostics{},
		},
		{
			name: "all values",
			input: []byte(
				`{"id":1,"organization":2,"url":"/inventories/1/","name":"my inventory","description":"My Test Inventory","variables":"{\"foo\":\"bar\"}"}`,
			),
			expected: InventoryDataSourceModel{
				Id:           types.Int64Value(1),
				Organization: types.Int64Value(2),
				Url:          types.StringValue("/inventories/1/"),
				Name:         types.StringValue("my inventory"),
				Description:  types.StringValue("My Test Inventory"),
				Variables:    jsontypes.NewNormalizedValue("{\"foo\":\"bar\"}"),
			},
			errors: diag.Diagnostics{},
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			resource := InventoryDataSourceModel{}
			diags := resource.ParseHttpResponse(test.input)
			if !test.errors.Equal(diags) {
				t.Errorf("Expected error diagnostics (%s), Received (%s)", test.errors, diags)
			}
			if test.expected != resource {
				t.Errorf("Expected (%s) not equal to actual (%s)", test.expected, resource)
			}
		})
	}
}

func TestAccInventoryDataSource(t *testing.T) {
	var inventory InventoryAPIModel
	randomName := acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)
	updatedName := "Inventory " + randomName
	updatedDescription := "Test inventory"
	updatedVariables := "{\"foo\": \"bar\"}"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccInventoryDataSourceId(12),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckInventoryDataSourceExists("aap_inventory.testdata", &inventory),
					testAccCheckInventoryDataSourceValues(&inventory, "", "", ""),
				),
			},
			// Update and Read testing
			{
				Config: testAccInventoryResource(updatedName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckInventoryDataSourceExists("aap_inventory.test", &inventory),
					testAccCheckInventoryDataSourceValues(&inventory, updatedName, updatedDescription, updatedVariables),
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

// testAccCheckInventoryDataSourceExists queries the AAP API and retrieves the matching inventory.
func testAccCheckInventoryDataSourceExists(name string, inventory *InventoryAPIModel) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		inventoryResource, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("inventory (%s) not found in state", name)
		}

		inventoryDataResponseBody, err := testGetResource(inventoryResource.Primary.Attributes["id"])
		if err != nil {
			return err
		}

		err = json.Unmarshal(inventoryDataResponseBody, &inventory)
		if err != nil {
			return err
		}

		if inventory.Id != 12 {
			return fmt.Errorf("inventory expected : 12 Found: (%d) ", inventory.Id)
		}

		return nil
	}
}

// testAccCheckInventoryDataSourcesValues verifies that the provided inventory retrieved from AAP contains the expected values.
func testAccCheckInventoryDataSourceValues(inventory *InventoryAPIModel, name string, description string, variables string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if inventory.Id != 12 {
			return fmt.Errorf("bad inventory ID in AAP, expected 12, got: %dv", inventory.Id)
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

// testAccInventoryResource returns a configuration for an AAP Inventory with the provided name and all options.
func testAccInventoryResource(name string) string {
	return fmt.Sprintf(`
resource "aap_inventory" "test" {
  name = "%s"
  description = "A test inventory"
  variables = "{\"abc\": \"def\"}"
}`, name)
}

// testAccInventoryDataSourceId returns a configuration for an AAP Inventory with the provided id.
func testAccInventoryDataSourceId(id int64) string {
	return fmt.Sprintf(`
data "aap_inventory" "testdata" {
  id = "%d"
}`, id)
}
