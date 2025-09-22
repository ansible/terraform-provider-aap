package provider

import (
	"context"
	fwaction "github.com/hashicorp/terraform-plugin-framework/action"
	"testing"
)

func TestEDAEventStreamActionSchema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schemaRequest := fwaction.SchemaRequest{}
	schemaResponse := &fwaction.SchemaResponse{}

	NewEDAEventStreamAction().Schema(ctx, schemaRequest, schemaResponse)

	if schemaResponse.Diagnostics.HasError() {
		t.Fatalf("Schema method diagnostics: %+v", schemaResponse.Diagnostics)
	}

	// Validate the schema
	diagnostics := schemaResponse.Schema.ValidateImplementation(ctx)

	if diagnostics.HasError() {
		t.Fatalf("Schema validation diagnostics: %+v", diagnostics)
	}
}
