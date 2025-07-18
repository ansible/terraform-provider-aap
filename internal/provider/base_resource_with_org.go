package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	tfpath "github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

var (
	_ resource.Resource                     = &BaseResourceWithOrg{}
	_ resource.ResourceWithConfigure        = &BaseResourceWithOrg{}
	_ resource.ResourceWithConfigValidators = &BaseResourceWithOrg{}
	_ resource.ResourceWithValidateConfig   = &BaseResourceWithOrg{}
)

// NewBaseResourceWithOrg creates a new instance of BaseResourceWithOrg.
func NewBaseResourceWithOrg(client ProviderHTTPClient, stringDescriptions StringDescriptions) *BaseResourceWithOrg {
	return &BaseResourceWithOrg{
		BaseResource: BaseResource{
			client:             client,
			StringDescriptions: stringDescriptions,
		},
	}
}

// Schema describes what data is available in the resource's configuration, plan, and state.
func (r *BaseResourceWithOrg) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
}

// Create creates the resource and sets the Terraform state on success.
func (r *BaseResourceWithOrg) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
}

// Read refreshes the Terraform state with the latest resource data.
func (r *BaseResourceWithOrg) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
}

// Update updates the inventory resource and sets the updated Terraform state on success.
func (r *BaseResourceWithOrg) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
}

// Delete removes a resource.
func (r *BaseResourceWithOrg) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
}

// Configure adds the provider configured client to the resource.
func (r *BaseResourceWithOrg) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
}

// ConfigValidators returns a list of functions which will be performed during validation.
func (r *BaseResourceWithOrg) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.Any(
			resourcevalidator.AtLeastOneOf(
				tfpath.MatchRoot("id")),
			resourcevalidator.RequiredTogether(
				tfpath.MatchRoot("name"),
				tfpath.MatchRoot("organization_name")),
		),
	}
}

// ValidateConfig defines imperative validation for the resource.
func (r *BaseResourceWithOrg) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
}
