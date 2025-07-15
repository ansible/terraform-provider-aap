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

func TestBaseDataSourceConfigValidators(t *testing.T) {
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
			name:       "organization datasource",
			datasource: NewOrganizationDataSource().(fwdatasource.DataSourceWithConfigValidators),
			expected: []fwdatasource.ConfigValidator{
				datasourcevalidator.Any(
					datasourcevalidator.AtLeastOneOf(
						tfpath.MatchRoot("id"),
						tfpath.MatchRoot("name")),
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
