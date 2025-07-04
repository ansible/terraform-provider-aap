package provider

import (
	"context"
	"testing"

	"github.com/ansible/terraform-provider-aap/internal/provider/customtypes"
	"github.com/ansible/terraform-provider-aap/internal/provider/mock_provider"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"go.uber.org/mock/gomock"
)

// Note: Once we have BaseResource implemented we can remove these mock constructors.

// NewMockGroupResource is a helper function to simplify the provider implementation.
func NewMockGroupResource(client ProviderHTTPClient) fwresource.Resource {
	return &GroupResource{
		client: client,
	}
}

// NewMockGroupResource is a helper function to simplify the provider implementation.
func NewMockHostResource(client ProviderHTTPClient) fwresource.Resource {
	return &HostResource{
		client: client,
	}
}

// NewIMocknventoryResource is a helper function to simplify the provider implementation.
func NewMockInventoryResource(client ProviderHTTPClient) fwresource.Resource {
	return &InventoryResource{
		client: client,
	}
}

// NewJobResource is a helper function to simplify the provider implementation.
func NewMockJobResource(client ProviderHTTPClient) fwresource.Resource {
	return &JobResource{
		client: client,
	}
}

func NewMockWorkflowJobResource(client ProviderHTTPClient) fwresource.Resource {
	return &WorkflowJobResource{
		client: client,
	}
}

func TestReadBaseResource(t *testing.T) {
	var diags diag.Diagnostics
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	mockclient := mock_provider.NewMockProviderHTTPClient(ctrl)

	testVariables := customtypes.NewAAPCustomStringValue("")
	testVariablesValue, err := testVariables.ToTerraformValue(ctx)
	jsonResponse := []byte(`{"id":1,"organization":2,"url":"/inventories/1/"}`)
	if err != nil {
		t.Error(err)
	}

	var testTable = []struct {
		name     string
		resource fwresource.Resource
		tfstate  tftypes.Value
		calls    func()
	}{
		{
			name:     "group resource",
			resource: NewMockGroupResource(mockclient),
			tfstate: tftypes.NewValue(
				tftypes.Object{
					AttributeTypes: map[string]tftypes.Type{
						"inventory_id": tftypes.Number,
						"id":           tftypes.Number,
						"name":         tftypes.String,
						"description":  tftypes.String,
						"url":          tftypes.String,
						"variables":    tftypes.DynamicPseudoType,
					},
				},
				map[string]tftypes.Value{
					"id":           tftypes.NewValue(tftypes.Number, int64(1)),
					"name":         tftypes.NewValue(tftypes.String, ""),
					"description":  tftypes.NewValue(tftypes.String, ""),
					"url":          tftypes.NewValue(tftypes.String, ""),
					"variables":    testVariablesValue,
					"inventory_id": tftypes.NewValue(tftypes.Number, int64(1)),
				},
			),
			calls: func() {
				mockclient.EXPECT().Get(gomock.Any()).Return(jsonResponse, diags)
			},
		},
		{
			name:     "host resource",
			resource: NewMockHostResource(mockclient),
			tfstate: tftypes.NewValue(
				tftypes.Object{
					AttributeTypes: map[string]tftypes.Type{
						"inventory_id": tftypes.Number,
						"id":           tftypes.Number,
						"name":         tftypes.String,
						"description":  tftypes.String,
						"url":          tftypes.String,
						"variables":    tftypes.DynamicPseudoType,
						"enabled":      tftypes.Bool,
						"groups":       tftypes.DynamicPseudoType,
					},
				},
				map[string]tftypes.Value{
					"id":           tftypes.NewValue(tftypes.Number, int64(1)),
					"name":         tftypes.NewValue(tftypes.String, ""),
					"description":  tftypes.NewValue(tftypes.String, ""),
					"url":          tftypes.NewValue(tftypes.String, ""),
					"variables":    testVariablesValue,
					"inventory_id": tftypes.NewValue(tftypes.Number, int64(1)),
					"enabled":      tftypes.NewValue(tftypes.Bool, true),
					"groups":       tftypes.NewValue(tftypes.Set{ElementType: tftypes.Number}, []tftypes.Value{tftypes.NewValue(tftypes.Number, int64(1))}),
				},
			),
			calls: func() {
				mockclient.EXPECT().Get(gomock.Any()).Return(jsonResponse, diags).Times(2)
			},
		},
		{
			name:     "inventory resource",
			resource: NewMockInventoryResource(mockclient),
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
			calls: func() {
				mockclient.EXPECT().Get(gomock.Any()).Return(jsonResponse, diags)
			},
		},
		{
			name:     "job resource",
			resource: NewMockJobResource(mockclient),
			tfstate: tftypes.NewValue(
				tftypes.Object{
					AttributeTypes: map[string]tftypes.Type{
						"inventory_id":                        tftypes.Number,
						"job_template_id":                     tftypes.Number,
						"job_type":                            tftypes.String,
						"status":                              tftypes.String,
						"url":                                 tftypes.String,
						"extra_vars":                          tftypes.DynamicPseudoType,
						"triggers":                            tftypes.DynamicPseudoType,
						"ignored_fields":                      tftypes.DynamicPseudoType,
						"wait_for_completion":                 tftypes.Bool,
						"wait_for_completion_timeout_seconds": tftypes.Number,
					},
				},
				map[string]tftypes.Value{
					"inventory_id":                        tftypes.NewValue(tftypes.Number, int64(1)),
					"job_template_id":                     tftypes.NewValue(tftypes.Number, int64(1)),
					"job_type":                            tftypes.NewValue(tftypes.String, ""),
					"status":                              tftypes.NewValue(tftypes.String, ""),
					"url":                                 tftypes.NewValue(tftypes.String, ""),
					"extra_vars":                          testVariablesValue,
					"triggers":                            tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, map[string]tftypes.Value{"test": tftypes.NewValue(tftypes.String, "")}),
					"ignored_fields":                      tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, []tftypes.Value{tftypes.NewValue(tftypes.String, "")}),
					"wait_for_completion":                 tftypes.NewValue(tftypes.Bool, true),
					"wait_for_completion_timeout_seconds": tftypes.NewValue(tftypes.Number, int64(1)),
				},
			),
			calls: func() {
				mockclient.EXPECT().GetWithStatus(gomock.Any()).Return(jsonResponse, diags, 200)
			},
		},
		{
			name:     "workflow job resource",
			resource: NewMockWorkflowJobResource(mockclient),
			tfstate: tftypes.NewValue(
				tftypes.Object{
					AttributeTypes: map[string]tftypes.Type{
						"inventory_id":             tftypes.Number,
						"workflow_job_template_id": tftypes.Number,
						"job_type":                 tftypes.String,
						"status":                   tftypes.String,
						"url":                      tftypes.String,
						"extra_vars":               tftypes.DynamicPseudoType,
						"triggers":                 tftypes.DynamicPseudoType,
						"ignored_fields":           tftypes.DynamicPseudoType,
					},
				},
				map[string]tftypes.Value{
					"inventory_id":             tftypes.NewValue(tftypes.Number, int64(1)),
					"workflow_job_template_id": tftypes.NewValue(tftypes.Number, int64(1)),
					"job_type":                 tftypes.NewValue(tftypes.String, ""),
					"status":                   tftypes.NewValue(tftypes.String, ""),
					"url":                      tftypes.NewValue(tftypes.String, ""),
					"extra_vars":               testVariablesValue,
					"triggers":                 tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, map[string]tftypes.Value{"test": tftypes.NewValue(tftypes.String, "")}),
					"ignored_fields":           tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, []tftypes.Value{tftypes.NewValue(tftypes.String, "")}),
				},
			),
			calls: func() {
				mockclient.EXPECT().Get(gomock.Any()).Return(jsonResponse, diags)
			},
		},
	}
	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			// Get Schema from Datasource
			schemaRequest := fwresource.SchemaRequest{}
			schemaResponse := &fwresource.SchemaResponse{}
			test.resource.Schema(ctx, schemaRequest, schemaResponse)

			// Build Read request and response objects
			readRequest := fwresource.ReadRequest{
				State: tfsdk.State{
					Raw:    test.tfstate,
					Schema: schemaResponse.Schema,
				},
			}
			readResponse := &fwresource.ReadResponse{
				State: tfsdk.State{
					Raw:    test.tfstate,
					Schema: schemaResponse.Schema,
				},
			}

			// Mock AAPClient Calls
			if test.calls != nil {
				test.calls()
			}

			// Append errors we encountered
			if diags.HasError() {
				readResponse.Diagnostics.Append(diags...)
			}

			//Call Read
			test.resource.Read(ctx, readRequest, readResponse)

			// Fail if we encountered an error during test
			if readResponse.Diagnostics != nil {
				if readResponse.Diagnostics.HasError() {
					t.Fatalf("ReadResponse diagnostics has error: %+v", readResponse.Diagnostics)
				}
			}

		})
	}
}
