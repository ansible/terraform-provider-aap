package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"path"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &BaseEdaDataSource{}
	_ datasource.DataSourceWithConfigure = &BaseEdaDataSource{}
)

// NewBaseEdaDataSource creates a new `BaseEdaDataSource`.
func NewBaseEdaDataSource(client ProviderHTTPClient, stringDescriptions StringDescriptions) *BaseEdaDataSource {
	return &BaseEdaDataSource{
		client:             client,
		StringDescriptions: stringDescriptions,
	}
}

// Metadata returns the data source type name composing it from the provider type name and the
// entity slug string passed in the constructor.
func (d *BaseEdaDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = fmt.Sprintf("%s_%s", req.ProviderTypeName, d.MetadataEntitySlug)
}

// GetBaseAttributes returns the base set of attributes for an EDA data source. This
// function is intended to be used by resource types that inherit from BaseEdaDatasource.
func (d *BaseEdaDataSource) GetBaseAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.Int64Attribute{
			Computed:    true,
			Description: fmt.Sprintf("%s id", d.DescriptiveEntityName),
		},
		"name": schema.StringAttribute{
			Required:    true,
			Description: fmt.Sprintf("Name of the %s", d.DescriptiveEntityName),
		},
		"url": schema.StringAttribute{
			Computed:    true,
			Description: fmt.Sprintf("URL of the %s", d.DescriptiveEntityName),
		},
	}
}

// Schema defines the schema fields for the data source.
func (d *BaseEdaDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes:  d.GetBaseAttributes(),
		Description: fmt.Sprintf("Gets an existing %s.", d.DescriptiveEntityName),
	}
}

// Configure adds the provider configured client to the data source.
func (d *BaseEdaDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Check that the response and diagnostics pointer is defined
	if resp == nil {
		tflog.Error(ctx, "Response not defined, we cannot continue with the execution")
		return
	}

	// Check that the current context is active
	if !IsContextActive("Configure", ctx, &resp.Diagnostics) {
		return
	}

	// Check that the provider data is configured
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*AAPClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *AAPClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = client
}

// Read refreshes the Terraform state with the latest data.
func (d *BaseEdaDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state BaseEdaSourceModel
	var diags diag.Diagnostics

	// Check Read preconditions
	if !DoReadPreconditionsMeet(ctx, resp, d.client) {
		return
	}

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)

	// Create the EDA path with query parameters
	edaEndpoint := d.client.getEdaApiEndpoint()
	if edaEndpoint == "" {
		resp.Diagnostics.AddError(
			"EDA API Endpoint is empty",
			"Expected a valid endpoint but was an empty string. Please report this issue to the provider developers.",
		)
		return
	}
	resourceURL := path.Join(edaEndpoint, d.ApiEntitySlug)
	params := map[string]string{
		"name": state.Name.ValueString(),
	}
	responseBody, diags := d.client.GetWithParams(resourceURL, params)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = state.ParseHttpResponse(responseBody)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// ParseHttpResponse parses an API response into a BaseEdaSourceModel instance.
func (d *BaseEdaSourceModel) ParseHttpResponse(body []byte) diag.Diagnostics {
	var diags diag.Diagnostics

	// Unmarshal the JSON response
	var apiModelList BaseEdaAPIModelList
	err := json.Unmarshal(body, &apiModelList)
	if err != nil {
		diags.AddError("Error parsing JSON response from AAP", err.Error())
		return diags
	}

	if len(apiModelList.Results) != 1 {
		diags.AddError("No event streams found in AAP", fmt.Sprintf("Expected 1 object in JSON response, found %d", len(apiModelList.Results)))
		return diags
	}

	var apiModel = apiModelList.Results[0]

	d.ID = types.Int64Value(apiModel.Id)
	d.URL = ParseStringValue(apiModel.URL)
	d.Name = ParseStringValue(apiModel.Name)
	return diags
}
