package provider

import (
	"bytes"
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

func TestHostResourceSchema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schemaRequest := fwresource.SchemaRequest{}
	schemaResponse := &fwresource.SchemaResponse{}

	// Instantiate the HostResource and call its Schema method
	NewHostResource().Schema(ctx, schemaRequest, schemaResponse)

	if schemaResponse.Diagnostics.HasError() {
		t.Fatalf("Schema method diagnostics: %+v", schemaResponse.Diagnostics)
	}

	// Validate the schema
	diagnostics := schemaResponse.Schema.ValidateImplementation(ctx)

	if diagnostics.HasError() {
		t.Fatalf("Schema validation diagnostics: %+v", diagnostics)
	}
}

func TestHostResourceCreateRequestBody(t *testing.T) {
	var testTable = []struct {
		name     string
		input    HostResourceModel
		expected []byte
	}{
		{
			name: "test with unknown values",
			input: HostResourceModel{
				Name:              types.StringValue("test host"),
				Description:       types.StringUnknown(),
				URL:               types.StringUnknown(),
				Variables:         jsontypes.NewNormalizedUnknown(),
				GroupId:           types.Int64Unknown(),
				DisassociateGroup: basetypes.NewBoolValue(false),
				Enabled:           basetypes.NewBoolValue(false),
				InventoryId:       types.Int64Unknown(),
				InstanceId:        types.Int64Unknown(),
			},
			expected: []byte(`{"enabled":false,"instance_id":0,"inventory":0,"name":"test host"}`),
		},
		{
			name: "test with null values",
			input: HostResourceModel{
				Name:              types.StringValue("test host"),
				Description:       types.StringNull(),
				URL:               types.StringNull(),
				Variables:         jsontypes.NewNormalizedNull(),
				GroupId:           types.Int64Null(),
				DisassociateGroup: basetypes.NewBoolValue(false),
				Enabled:           basetypes.NewBoolValue(false),
				InventoryId:       types.Int64Null(),
				InstanceId:        types.Int64Null(),
			},
			expected: []byte(`{"enabled":false,"instance_id":0,"inventory":0,"name":"test host"}`),
		},
		{
			name: "test with some values",
			input: HostResourceModel{
				Name:        types.StringValue("host1"),
				Description: types.StringNull(),
				URL:         types.StringValue("/api/v2/hosts/1/"),
				Variables:   jsontypes.NewNormalizedValue("{\"foo\":\"bar\"}"),
			},
			expected: []byte(
				`{"instance_id":0,"inventory":0,"name":"host1","url":"/api/v2/hosts/1/",` +
					`"variables":"{\"foo\":\"bar\"}"}`,
			),
		},
		{
			name: "test with group id",
			input: HostResourceModel{
				Name:        types.StringValue("host1"),
				Description: types.StringNull(),
				URL:         types.StringValue("/api/v2/hosts/1/"),
				Variables:   jsontypes.NewNormalizedValue("{\"foo\":\"bar\"}"),
				GroupId:     basetypes.NewInt64Value(2),
			},
			expected: []byte(
				`{"id":2,"instance_id":0,"inventory":0,"name":"host1","url":"/api/v2/hosts/1/",` +
					`"variables":"{\"foo\":\"bar\"}"}`,
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

// CustomError is a custom error type
type CustomError struct {
	Message string
}

// Implement the error interface for Cu
func (e CustomError) Error() string {
	return e.Message
}

func TestHostResourceParseHttpResponse(t *testing.T) {
	customErr := CustomError{
		Message: "invalid character 'N' looking for beginning of value",
	}
	emptyError := CustomError{}

	var testTable = []struct {
		name     string
		input    []byte
		expected HostResourceModel
		errors   error
	}{
		{
			name:     "test with JSON error",
			input:    []byte("Not valid JSON"),
			expected: HostResourceModel{},
			errors:   customErr,
		},
		{
			name:  "test with missing values",
			input: []byte(`{"name": "host1", "url": "/api/v2/hosts/1/", "description": "", "variables": "", "group_id": 2}`),
			expected: HostResourceModel{
				Name:        types.StringValue("host1"),
				URL:         types.StringValue("/api/v2/hosts/1/"),
				Description: types.StringNull(),
				GroupId:     types.Int64Value(2),
				Variables:   jsontypes.NewNormalizedNull(),
			},
			errors: emptyError,
		},
		{
			name: "test with all values",
			input: []byte(
				`{"description":"A basic test host","group_id":1,"name":"host1","disassociate_group":false,` +
					`"enabled":false,"url":"/api/v2/hosts/1/","variables":"{\"foo\":\"bar\",\"nested\":{\"foobar\":\"baz\"}}"}`,
			),
			expected: HostResourceModel{
				Name:              types.StringValue("host1"),
				URL:               types.StringValue("/api/v2/hosts/1/"),
				Description:       types.StringValue("A basic test host"),
				GroupId:           types.Int64Value(1),
				DisassociateGroup: basetypes.NewBoolValue(false),
				Variables:         jsontypes.NewNormalizedValue("{\"foo\":\"bar\",\"nested\":{\"foobar\":\"baz\"}}"),
				Enabled:           basetypes.NewBoolValue(false),
			},
			errors: emptyError,
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			resource := HostResourceModel{}
			err := resource.ParseHttpResponse(test.input)
			if test.errors != nil && err != nil && test.errors.Error() != err.Error() {
				t.Errorf("Expected error diagnostics (%s), actual was (%s)", test.errors, err)
			}
			if test.expected != resource {
				t.Errorf("Expected (%s) not equal to actual (%s)", test.expected, resource)
			}
		})
	}
}
