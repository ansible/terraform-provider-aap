package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path"

	"github.com/ansible/terraform-provider-aap/internal/provider/customtypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	tfpath "github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	tftypes "github.com/hashicorp/terraform-plugin-framework/types"
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
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:    true,
				Description: fmt.Sprintf("%s id", r.DescriptiveEntityName),
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"organization": schema.Int64Attribute{
				Computed:    true,
				Optional:    true,
				Description: fmt.Sprintf("Identifier for the organization to which the %s belongs", r.DescriptiveEntityName),
			},
			"organization_name": schema.StringAttribute{
				Optional:    true,
				Description: fmt.Sprintf("The name for the organization to which the %s belongs", r.DescriptiveEntityName),
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
func (r *BaseResourceWithOrg) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var state BaseResourceModelWithOrg
	var diags diag.Diagnostics

	// Read Terraform plan data into resource model
	diags = req.Plan.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Create request body from resource data
	requestBytes, diags := state.CreateRequestBody()
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	requestBody := bytes.NewReader(requestBytes)

	// Generate the uri for creating the resource
	uri := path.Join(r.client.getApiEndpoint(), r.ApiEntitySlug)
	resourceURL, err := state.CreateNamedURL(uri, &BaseDetailAPIModelWithOrg{
		BaseDetailAPIModel: BaseDetailAPIModel{
			Id:   state.Id.ValueInt64(),
			Name: state.Name.ValueString(),
		},
		Organization: state.Organization.ValueInt64(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Minimal Data Not Supplied", "Expected [id] or [name and organization_name]")
	}

	// Create the new resource
	createResponseBody, diags := r.client.Create(resourceURL, requestBody)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Parse the response from the AAP API
	diags = state.ParseHttpResponse(createResponseBody)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
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

func (r *BaseResourceModelWithOrg) CreateRequestBody() ([]byte, diag.Diagnostics) {
	resource := BaseDetailAPIModelWithOrg{
		BaseDetailAPIModel: BaseDetailAPIModel{
			Id:          r.Id.ValueInt64(),
			URL:         r.URL.String(),
			Description: r.Description.String(),
			Name:        r.Name.String(),
			Variables:   r.Variables.String(),
		},
		Organization: r.Organization.ValueInt64(),
	}

	jsonBody, err := json.Marshal(resource)
	if err != nil {
		var diags diag.Diagnostics
		diags.AddError(
			"Error marshalling request body",
			fmt.Sprintf("Could not create request body for resource, unexpected error: %s", err.Error()),
		)
		return nil, diags
	}
	return jsonBody, nil
}

func (r *BaseResourceModelWithOrg) ParseHttpResponse(body []byte) diag.Diagnostics {
	// Use parent parse function to cover the base fields
	diags := r.BaseResourceModel.ParseHttpResponse(body)
	if diags.HasError() {
		return diags
	}

	// Unmarshal the response
	var apiModel BaseDetailAPIModelWithOrg
	err := json.Unmarshal(body, &apiModel)
	if err != nil {
		diags.AddError("Error parsing JSON response from AAP", err.Error())
		return diags
	}

	// Copy values into the BaseDetailResourceModeWithOrg
	r.Organization = tftypes.Int64Value(apiModel.Organization)
	r.OrganizationName = ParseStringValue(apiModel.SummaryFields.Organization.Name)

	return diags
}
