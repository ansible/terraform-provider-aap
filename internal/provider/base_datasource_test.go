package provider

import (
	"context"
	"reflect"
	"testing"

	"github.com/ansible/terraform-provider-aap/internal/provider/customtypes"
	"github.com/ansible/terraform-provider-aap/internal/provider/mock_provider"
	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	fwdatasource "github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	tfpath "github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"go.uber.org/mock/gomock"
)

func TestBaseDataSourceMetadata(t *testing.T) {
	t.Parallel()

	testDataSource := NewBaseDataSource(nil, StringDescriptions{
		ApiEntitySlug:         "datasource",
		DescriptiveEntityName: "datasource",
		MetadataEntitySlug:    "datasource",
	})
	ctx := context.Background()
	metadataRequest := fwdatasource.MetadataRequest{
		ProviderTypeName: "provider",
	}
	metadataResponse := &fwdatasource.MetadataResponse{}
	expected := "provider_datasource"

	testDataSource.Metadata(ctx, metadataRequest, metadataResponse)
	if metadataResponse.TypeName != expected {
		t.Errorf("Expected (%s) not equal to actual (%s)", expected, metadataResponse.TypeName)
	}
}

func TestBaseDataSourceSchema(t *testing.T) {
	t.Parallel()

	testDataSource := NewBaseDataSource(nil, StringDescriptions{
		ApiEntitySlug:         "datasource",
		DescriptiveEntityName: "datasource",
		MetadataEntitySlug:    "datasource",
	})
	ctx := context.Background()
	schemaRequest := fwdatasource.SchemaRequest{}
	schemaResponse := &fwdatasource.SchemaResponse{}

	testDataSource.Schema(ctx, schemaRequest, schemaResponse)

	if schemaResponse.Diagnostics.HasError() {
		t.Fatalf("Schema method diagnostics: %+v", schemaResponse.Diagnostics)
	}

	// Validate the schema
	diagnostics := schemaResponse.Schema.ValidateImplementation(ctx)

	if diagnostics.HasError() {
		t.Fatalf("Schema validation diagnostics: %+v", diagnostics)
	}
}

func TestDataSourceConfigValidators(t *testing.T) {
	t.Parallel()

	var testTable = []struct {
		name       string
		datasource fwdatasource.DataSourceWithConfigValidators
		expected   []fwdatasource.ConfigValidator
	}{
		{
			name: "base datasource",
			datasource: NewBaseDataSource(nil, StringDescriptions{
				ApiEntitySlug:         "datasource",
				DescriptiveEntityName: "datasource",
				MetadataEntitySlug:    "datasource",
			}),
			expected: []fwdatasource.ConfigValidator{
				datasourcevalidator.Any(
					datasourcevalidator.AtLeastOneOf(
						tfpath.MatchRoot("id")),
				),
			},
		},
		{
			name: "base datasource with org",
			datasource: NewBaseDataSourceWithOrg(nil, StringDescriptions{
				ApiEntitySlug:         "datasource",
				DescriptiveEntityName: "datasource",
				MetadataEntitySlug:    "datasource",
			}),
			expected: []fwdatasource.ConfigValidator{
				datasourcevalidator.Any(
					datasourcevalidator.AtLeastOneOf(
						tfpath.MatchRoot("id")),
					datasourcevalidator.RequiredTogether(
						tfpath.MatchRoot("name"),
						tfpath.MatchRoot("organization_name")),
				),
			},
		},
	}
	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			validator := test.datasource.ConfigValidators(ctx)
			if !reflect.DeepEqual(validator, test.expected) {
				t.Errorf("Expected (%s) not equal to actual (%s)", test.expected, validator)
			}
		})
	}
}

func TestReadBaseDataSource(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	mockclient := mock_provider.NewMockProviderHTTPClient(ctrl)

	testVariables := customtypes.NewAAPCustomStringValue("")
	testVariablesValue, err := testVariables.ToTerraformValue(ctx)
	if err != nil {
		t.Error(err)
	}

	var testTable = []struct {
		name       string
		datasource fwdatasource.DataSource
		tfstate    tftypes.Value
	}{
		{
			name: "base datasource",
			datasource: NewBaseDataSource(mockclient, StringDescriptions{
				ApiEntitySlug:         "inventories",
				DescriptiveEntityName: "Inventory",
				MetadataEntitySlug:    "inventory",
			}),
			tfstate: tftypes.NewValue(
				tftypes.Object{
					AttributeTypes: map[string]tftypes.Type{
						"id":          tftypes.Number,
						"name":        tftypes.String,
						"description": tftypes.String,
						"url":         tftypes.String,
						"named_url":   tftypes.String,
						"variables":   tftypes.DynamicPseudoType,
					},
				},
				map[string]tftypes.Value{
					"id":          tftypes.NewValue(tftypes.Number, int64(1)),
					"name":        tftypes.NewValue(tftypes.String, ""),
					"description": tftypes.NewValue(tftypes.String, ""),
					"url":         tftypes.NewValue(tftypes.String, ""),
					"named_url":   tftypes.NewValue(tftypes.String, ""),
					"variables":   testVariablesValue,
				},
			),
		},
		{
			name: "base datasource with org",
			datasource: NewBaseDataSourceWithOrg(mockclient, StringDescriptions{
				ApiEntitySlug:         "inventories",
				DescriptiveEntityName: "Inventory",
				MetadataEntitySlug:    "inventory",
			}),
			tfstate: tftypes.NewValue(
				tftypes.Object{
					AttributeTypes: map[string]tftypes.Type{
						"id":                tftypes.Number,
						"name":              tftypes.String,
						"description":       tftypes.String,
						"url":               tftypes.String,
						"named_url":         tftypes.String,
						"variables":         tftypes.DynamicPseudoType,
						"organization":      tftypes.Number,
						"organization_name": tftypes.String,
					},
				},
				map[string]tftypes.Value{
					"id":                tftypes.NewValue(tftypes.Number, int64(1)),
					"name":              tftypes.NewValue(tftypes.String, ""),
					"description":       tftypes.NewValue(tftypes.String, ""),
					"url":               tftypes.NewValue(tftypes.String, ""),
					"named_url":         tftypes.NewValue(tftypes.String, ""),
					"variables":         testVariablesValue,
					"organization":      tftypes.NewValue(tftypes.Number, int64(1)),
					"organization_name": tftypes.NewValue(tftypes.String, ""),
				},
			),
		},
	}
	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			var diags diag.Diagnostics
			// Get Schema from Datasource
			schemaRequest := fwdatasource.SchemaRequest{}
			schemaResponse := &fwdatasource.SchemaResponse{}
			test.datasource.Schema(ctx, schemaRequest, schemaResponse)

			// Build Read request and response objects
			readRequest := fwdatasource.ReadRequest{
				Config: tfsdk.Config{
					Raw:    test.tfstate,
					Schema: schemaResponse.Schema,
				},
			}
			readResponse := &fwdatasource.ReadResponse{
				State: tfsdk.State{
					Raw:    test.tfstate,
					Schema: schemaResponse.Schema,
				},
			}
			jsonResponse := []byte(`{"id":1,"organization":2,"url":"/inventories/1/"}`)

			// Mock AAPClient Calls
			mockclient.EXPECT().GetApiEndpoint().Return("localhost:44925/api")
			mockclient.EXPECT().Get("localhost:44925/api/inventories/1").Return(jsonResponse, diags)

			// Append errors we encountered
			if diags.HasError() {
				readResponse.Diagnostics.Append(diags...)
			}

			//Call Read
			test.datasource.Read(ctx, readRequest, readResponse)

			// Fail if we encountered an error during test
			if readResponse.Diagnostics != nil {
				if readResponse.Diagnostics.HasError() {
					t.Fatalf("ReadResponse diagnostics has error: %+v", readResponse.Diagnostics)
				}
			}

		})
	}
}
