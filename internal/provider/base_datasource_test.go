package provider

import (
	"context"
	"testing"

	fwdatasource "github.com/hashicorp/terraform-plugin-framework/datasource"
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
