package provider

import (
	"context"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	fwdatasource "github.com/hashicorp/terraform-plugin-framework/datasource"
	tfpath "github.com/hashicorp/terraform-plugin-framework/path"
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

// func TestReadBaseDataSource(t *testing.T) {
// 	var diags diag.Diagnostics

// 	ctrl := gomock.NewController(t)
// 	mockclient := mock_provider.NewMockProviderHTTPClient(ctrl)

// 	testDataSource := NewBaseDataSource(mockclient, StringDescriptions{
// 		MetadataEntitySlug:    "inventory",
// 		DescriptiveEntityName: "Inventory",
// 		ApiEntitySlug:         "inventories",
// 	})
// 	ctx := context.Background()
// 	schemaRequest := fwdatasource.SchemaRequest{}
// 	schemaResponse := &fwdatasource.SchemaResponse{}
// 	testDataSource.Schema(ctx, schemaRequest, schemaResponse)

// 	testVariables := customtypes.NewAAPCustomStringValue("")
// 	testVariablesValue, err := testVariables.ToTerraformValue(ctx)
// 	if err != nil {
// 		t.Error(err)
// 	}
// 	rawStateObj := tftypes.NewValue(
// 		tftypes.Object{
// 			AttributeTypes: map[string]tftypes.Type{
// 				"id":          tftypes.Number,
// 				"name":        tftypes.String,
// 				"description": tftypes.String,
// 				"url":         tftypes.String,
// 				"named_url":   tftypes.String,
// 				"variables":   tftypes.DynamicPseudoType,
// 			},
// 		},
// 		map[string]tftypes.Value{
// 			"id":          tftypes.NewValue(tftypes.Number, int64(1)),
// 			"name":        tftypes.NewValue(tftypes.String, ""),
// 			"description": tftypes.NewValue(tftypes.String, ""),
// 			"url":         tftypes.NewValue(tftypes.String, ""),
// 			"named_url":   tftypes.NewValue(tftypes.String, ""),
// 			"variables":   testVariablesValue,
// 		},
// 	)

// 	readRequest := fwdatasource.ReadRequest{
// 		Config: tfsdk.Config{
// 			Raw:    rawStateObj,
// 			Schema: schemaResponse.Schema,
// 		},
// 	}
// 	readResponse := &fwdatasource.ReadResponse{
// 		State: tfsdk.State{
// 			Raw:    rawStateObj,
// 			Schema: schemaResponse.Schema,
// 		},
// 	}
// 	jsonResponse := []byte(
// 		`{"id":1,"organization":2,"url":"/inventories/1/","name":"my inventory","description":"My Test Inventory","variables":"{\"foo\":\"bar\"}"}`,
// 	)

// 	mockclient.EXPECT().GetApiEndpoint().Return("localhost:44925/api")
// 	mockclient.EXPECT().Get("localhost:44925/api/inventories/1").Return(jsonResponse, diags)
// 	testDataSource.Read(ctx, readRequest, readResponse)

// 	if readResponse.Diagnostics != nil {
// 		if readResponse.Diagnostics.HasError() {
// 			t.Fatalf("ReadResponse diagnostics has error: %+v", readResponse.Diagnostics)
// 		}
// 	}
// }
