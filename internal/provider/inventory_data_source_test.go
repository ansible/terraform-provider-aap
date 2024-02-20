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
	updatedDescription := "A test inventory"
	updatedVariables := "{\"abc\": \"def\"}"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create an inventory using the Inventory resource
			{
				Config: testAccInventoryDataSource(randomName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckInventoryDataSourceExists("aap_inventory.test", &inventory),
					testAccCheckInventoryDataSourceValues(&inventory, randomName, updatedDescription, updatedVariables),
				),
			},
			// Step 2: Read the inventory using the Inventory Data Source
			{
				Config: testAccInventoryDataSource(randomName),
				Check: resource.ComposeAggregateTestCheckFunc(

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

		if inventory.Id == 0 {
			return fmt.Errorf("inventory (%s) not found in AAP", inventoryResource.Primary.ID)
		}

		return nil
	}
}

// testAccCheckInventoryDataSourcesValues verifies that the provided inventory retrieved from AAP contains the expected values.
func testAccCheckInventoryDataSourceValues(inventory *InventoryAPIModel, expectedName, expectedDescription, expectedVariables string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if inventory.Id == 0 {
			return fmt.Errorf("bad inventory ID in AAP, expected a positive int64, got: %dv", inventory.Id)
		}
		if inventory.Organization == 0 {
			return fmt.Errorf("bad inventory organization in AAP, expected a positive int64, got: %d", inventory.Organization)
		}
		if inventory.Url == "" {
			return fmt.Errorf("bad inventory URL in AAP, expected a URL path, got: %s", inventory.Url)
		}
		if inventory.Name != expectedName {
			return fmt.Errorf("bad inventory name in AAP, expected \"%s\", got: %s", expectedName, inventory.Name)
		}
		if inventory.Description != expectedDescription {
			return fmt.Errorf("bad inventory description in AAP, expected \"%s\", got: %s", expectedDescription, inventory.Description)
		}
		if inventory.Variables != expectedVariables {
			return fmt.Errorf("bad inventory variables in AAP, expected \"%s\", got: %s", expectedVariables, inventory.Variables)
		}
		return nil
	}
}

// testAccInventoryDataSource configures the Inventory Data Source for testing
func testAccInventoryDataSource(name string) string {
	return fmt.Sprintf(`
resource "aap_inventory" "test" {
  name        = "%s"
  organization = 1
  description = "A test inventory"
  variables   = "{\"abc\": \"def\"}"
}

data "aap_inventory" "test" {
  id = aap_inventory.test.id
}
`, name)
}
