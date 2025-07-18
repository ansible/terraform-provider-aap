package provider

import (
	"context"
	"fmt"

	"github.com/ansible/terraform-provider-aap/internal/provider/customtypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	tfpath "github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
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
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Optional:    true,
				Description: fmt.Sprintf("%s id", r.DescriptiveEntityName),
			},
			"url": schema.StringAttribute{
				Computed:    true,
				Description: fmt.Sprintf("Url of the %s", r.DescriptiveEntityName),
			},
			"named_url": schema.StringAttribute{
				Computed:    true,
				Description: fmt.Sprintf("The Named Url of the %s", r.DescriptiveEntityName),
			},
			"name": schema.StringAttribute{
				Optional:    true,
				Description: fmt.Sprintf("Name of the %s", r.DescriptiveEntityName),
			},
			"description": schema.StringAttribute{
				Computed:    true,
				Description: fmt.Sprintf("Description of the %s", r.DescriptiveEntityName),
			},
			"variables": schema.StringAttribute{
				Computed:   true,
				CustomType: customtypes.AAPCustomStringType{},
				Description: fmt.Sprintf("Variables of the %s. Will be either JSON or YAML depending on how the "+
					"variables were entered into AAP.", r.DescriptiveEntityName),
				DeprecationMessage: "This attribute is deprecated and will be removed in a future version.",
			},
		},
		Description: fmt.Sprintf("Get an existing %s.", r.DescriptiveEntityName),
	}
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
