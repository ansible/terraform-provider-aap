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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	tftypes "github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
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
				Computed:    true,
				Description: fmt.Sprintf("%s id", r.DescriptiveEntityName),
			},
			"url": schema.StringAttribute{
				Computed:    true,
				Description: fmt.Sprintf("Url of the %s", r.DescriptiveEntityName),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"named_url": schema.StringAttribute{
				Computed:    true,
				Description: fmt.Sprintf("The Named Url of the %s", r.DescriptiveEntityName),
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: fmt.Sprintf("Name of the %s", r.DescriptiveEntityName),
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: fmt.Sprintf("Description of the %s", r.DescriptiveEntityName),
			},
			"variables": schema.StringAttribute{
				Optional:           true,
				CustomType:         customtypes.AAPCustomStringType{},
				Description:        fmt.Sprintf("%s variables. Must be provided as either a JSON or YAML string", r.DescriptiveEntityName),
				DeprecationMessage: "This attribute is deprecated and will be removed in a future version.",
			},
		},
		Description: fmt.Sprintf("Creates a %s.", r.DescriptiveEntityName),
	}
}

// Create creates the resource and sets the Terraform state on success.
func (r *BaseResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var state BaseResourceModel
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

	// Create the uri to create the resource
	uri := path.Join(r.client.getApiEndpoint(), r.ApiEntitySlug)
	resourceURL, err := state.CreateNamedURL(uri, &BaseDetailAPIModel{
		Id:   state.Id.ValueInt64(),
		Name: state.Name.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Minimal Data Not Supplied", "Expected [id]")
		return
	}

	// Create new resource
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

	// Save new resource data into resource model
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Read refreshes the Terraform state with the latest resource data.
func (r *BaseResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state BaseResourceModel
	var diags diag.Diagnostics

	// Check preconditions
	if !DoReadPreconditionsMeet(ctx, resp, r.client) {
		return
	}

	// Read Terraform configuration into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	uri := path.Join(r.client.getApiEndpoint(), r.ApiEntitySlug)
	resourceURL, err := state.CreateNamedURL(uri, &BaseDetailAPIModel{
		Id:   state.Id.ValueInt64(),
		Name: state.Name.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Minimal Data Not Supplied", "Expected [id]")
		return
	}

	// Make the API call
	apiResponse, diags := r.client.Get(resourceURL)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Parse response from the API
	diags = state.ParseHttpResponse(apiResponse)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update updates the inventory resource and sets the updated Terraform state on success.
func (r *BaseResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
}

// Delete removes a resource.
func (r *BaseResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
}

// Configure adds the provider configured client to the resource.
func (r *BaseResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if resp == nil {
		tflog.Error(ctx, "Response not defined, we cannot continue with the execution.")
		return
	}

	if !IsContextActive("Configure", ctx, resp.Diagnostics) {
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

// generateRequestBody creates a JSON encoded request body from the resource data.
func (r *BaseResourceModel) CreateRequestBody() ([]byte, diag.Diagnostics) {
	resource := BaseDetailAPIModel{
		Id:          r.Id.ValueInt64(),
		URL:         r.URL.String(),
		Description: r.Description.String(),
		Name:        r.Name.String(),
		Variables:   r.Variables.String(),
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

func (r *BaseResourceModel) ParseHttpResponse(responseBody []byte) diag.Diagnostics {
	var diags diag.Diagnostics

	// Unmarshal the response body
	var apiModel BaseDetailAPIModel
	err := json.Unmarshal(responseBody, &apiModel)
	if err != nil {
		diags.AddError("Error parsing JSON response from AAP", err.Error())
		return diags
	}

	// Copy values into the BaseDetailResourceModel
	r.Id = tftypes.Int64Value(apiModel.Id)
	r.URL = ParseStringValue(apiModel.URL)
	r.Name = ParseStringValue(apiModel.Name)
	r.Description = ParseStringValue(apiModel.Description)
	r.NamedUrl = ParseStringValue(apiModel.Related.NamedUrl)
	r.Variables = ParseAAPCustomStringValue(apiModel.Variables)

	return diags
}
