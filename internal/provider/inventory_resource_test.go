package provider

import (
	"bytes"
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
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
		input    inventoryResourceModel
		expected []byte
	}{
		{
			name: "unknown values",
			input: inventoryResourceModel{
				Id:           types.Int64Unknown(),
				Organization: types.Int64Unknown(),
				Url:          types.StringUnknown(),
				Name:         types.StringValue("test inventory"),
				Description:  types.StringUnknown(),
				Variables:    jsontypes.NewNormalizedUnknown(),
			},
			expected: []byte(`{"organization":1,"name":"test inventory"}`),
		},
		{
			name: "null values",
			input: inventoryResourceModel{
				Id:           types.Int64Null(),
				Organization: types.Int64Null(),
				Url:          types.StringNull(),
				Name:         types.StringValue("test inventory"),
				Description:  types.StringNull(),
				Variables:    jsontypes.NewNormalizedNull(),
			},
			expected: []byte(`{"organization":1,"name":"test inventory"}`),
		},
		{
			name: "provided values",
			input: inventoryResourceModel{
				Id:           types.Int64Value(1),
				Organization: types.Int64Value(2),
				Url:          types.StringValue("/inventories/1/"),
				Name:         types.StringValue("test inventory"),
				Description:  types.StringValue("A test inventory for testing"),
				Variables:    jsontypes.NewNormalizedValue("{\"foo\": \"bar\", \"nested\": {\"foobar\": \"baz\"}}"),
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
		expected inventoryResourceModel
		errors   diag.Diagnostics
	}{
		{
			name:     "JSON error",
			input:    []byte("Not valid JSON"),
			expected: inventoryResourceModel{},
			errors:   jsonError,
		},
		{
			name:  "missing values",
			input: []byte(`{"id":1,"type":"inventory","name":"test inventory","organization":2,"url":"/inventories/1/"}`),
			expected: inventoryResourceModel{
				Id:           types.Int64Value(1),
				Organization: types.Int64Value(2),
				Url:          types.StringValue("/inventories/1/"),
				Name:         types.StringValue("test inventory"),
				Description:  types.StringNull(),
				Variables:    jsontypes.NewNormalizedNull(),
			},
			errors: diag.Diagnostics{},
		},
		{
			name: "all values",
			input: []byte(
				`{"description":"A test inventory for testing","id":1,"name":"test inventory","organization":2,` +
					`"type":"inventory","url":"/inventories/1/","variables":"{\"foo\":\"bar\",\"nested\":{\"foobar\":\"baz\"}}"}`,
			),
			expected: inventoryResourceModel{
				Id:           types.Int64Value(1),
				Organization: types.Int64Value(2),
				Url:          types.StringValue("/inventories/1/"),
				Name:         types.StringValue("test inventory"),
				Description:  types.StringValue("A test inventory for testing"),
				Variables:    jsontypes.NewNormalizedValue("{\"foo\":\"bar\",\"nested\":{\"foobar\":\"baz\"}}"),
			},
			errors: diag.Diagnostics{},
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			resource := inventoryResourceModel{}
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
