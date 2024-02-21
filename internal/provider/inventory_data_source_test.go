package provider

import (
	"context"
	"fmt"
	"testing"

	fwdatasource "github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestInventoryDataSourceSchema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schemaRequest := fwdatasource.SchemaRequest{}
	schemaResponse := &fwdatasource.SchemaResponse{}

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
				Variables:    types.StringNull(),
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
				Variables:    types.StringValue("{\"foo\":\"bar\"}"),
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

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create an inventory and Read
			{
				Config: testAccInventoryDataSource(randomName),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Check the existence of the created inventory and store its values in the inventory model
					testAccCheckInventoryResourceExists("aap_inventory.test", &inventory),
					// Verify the data source values against the stored resource values
					resource.TestCheckResourceAttrPtr("data.aap_inventory.test", "name", &inventory.Name),
					resource.TestCheckResourceAttr("data.aap_inventory.test", "organization", "1"),
					resource.TestCheckResourceAttrPtr("data.aap_inventory.test", "description", &inventory.Description),
					resource.TestCheckResourceAttrPtr("data.aap_inventory.test", "variables", &inventory.Variables),
					resource.TestCheckResourceAttrPtr("data.aap_inventory.test", "url", &inventory.Url),
				),
			},
		},
		CheckDestroy: testAccCheckInventoryResourceDestroy,
	})
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
