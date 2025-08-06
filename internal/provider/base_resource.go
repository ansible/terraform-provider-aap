package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// NewBaseResource creates a new instance of BaseResource.
func NewBaseResource(client ProviderHTTPClient, stringDescriptions StringDescriptions) *BaseResource {
	return &BaseResource{
		client:             client,
		StringDescriptions: stringDescriptions,
	}
}

// GetBaseAttributes returns the base set of attributes for a resource. This function
// is intended to be used by resource types that inherit from BaseResource.
func (r *BaseResource) GetBaseAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"url": schema.StringAttribute{
			Computed:    true,
			Description: fmt.Sprintf("Url of the %s", r.DescriptiveEntityName),
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
	}
}

// Metadata defines the resource name as it would appear in Terraform configurations.
// For resources in this project it is aap_<resourceName>, like aap_inventory.
func (r *BaseResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = fmt.Sprintf("%s_%s", req.ProviderTypeName, r.MetadataEntitySlug)
}

// Schema describes what data is available in the resource's configuration, plan, and state.
func (r *BaseResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes:  r.GetBaseAttributes(),
		Description: fmt.Sprintf("Creates a %s.", r.DescriptiveEntityName),
	}
}

// Configure adds the provider configured client to the resource.
func (r *BaseResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if resp == nil {
		tflog.Error(ctx, "Response not defined, we cannot continue with the execution.")
		return
	}

	if !IsContextActive("Configure", ctx, &resp.Diagnostics) {
		return
	}

	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*AAPClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Error",
			fmt.Sprintf("Expected *AAPClient, got %T. Please report this to the provider developers.", req.ProviderData))

		return
	}

	r.client = client
}
