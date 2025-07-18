package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	tfpath "github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

var (
	_ resource.Resource                     = &BaseResource{}
	_ resource.ResourceWithConfigure        = &BaseResource{}
	_ resource.ResourceWithConfigValidators = &BaseResource{}
	_ resource.ResourceWithValidateConfig   = &BaseResource{}
)

// NewBaseResource creates a new instance of BaseResource.
func NewBaseResource(client ProviderHTTPClient, stringDescriptions StringDescriptions) *BaseResource {
	return &BaseResource{
		client:             client,
		StringDescriptions: stringDescriptions,
	}
}

// Metadata defines the resource name as it would appear in Terraform configurations.
// For resources in this project it is aap_<resourceName>, like aap_inventory.
func (r *BaseResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = fmt.Sprintf("%s_%s", req.ProviderTypeName, r.MetadataEntitySlug)
}

// Schema describes what data is available in the resource's configuration, plan, and state.
func (r *BaseResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
}

// Create creates the resource and sets the Terraform state on success.
func (r *BaseResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
}

// Read refreshes the Terraform state with the latest resource data.
func (r *BaseResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
}

// Update updates the inventory resource and sets the updated Terraform state on success.
func (r *BaseResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
}

// Delete removes a resource.
func (r *BaseResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
}

// Configure adds the provider configured client to the resource.
func (r *BaseResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
}

// ConfigValidators returns a list of functions which will be performed during validation.
func (r *BaseResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.Any(
			resourcevalidator.AtLeastOneOf(
				tfpath.MatchRoot("id")),
		),
	}
}

// ValidateConfig defines imperative validation for the resource.
func (r *BaseResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
}
