package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"path"

	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	tfpath "github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource                     = &BaseEdaDataSource{}
	_ datasource.DataSourceWithConfigure        = &BaseEdaDataSource{}
	_ datasource.DataSourceWithConfigValidators = &BaseEdaDataSource{}
	_ datasource.DataSourceWithValidateConfig   = &BaseEdaDataSource{}
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
	fmt.Println(resp.TypeName)
}

// Schema defines the schema fields for the data source.
func (d *BaseEdaDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required: true,
			},
			"url": schema.StringAttribute{
				Computed: true,
			},
			"id": schema.Int64Attribute{
				Computed: true,
			},
		},
	}
}

// ConfigValidators validates configuration in a declarative way.
func (d *BaseEdaDataSource) ConfigValidators(_ context.Context) []datasource.ConfigValidator {
	// You have at least an name
	return []datasource.ConfigValidator{
		datasourcevalidator.Any(
			datasourcevalidator.AtLeastOneOf(
				tfpath.MatchRoot("name")),
		),
	}
}

// ValidateConfig validates configuration in an imperative way.
func (d *BaseEdaDataSource) ValidateConfig(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	// Check that the response and diagnostics pointer is defined
	if resp == nil {
		tflog.Error(ctx, "Response not defined, we cannot continue with the execution")
		return
	}

	// Check that the current context is active
	if !IsContextActive("ValidateConfig", ctx, &resp.Diagnostics) {
		return
	}

	var data BaseEdaSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if IsValueProvidedOrPromised(data.Name) {
		return
	}

	if !IsValueProvidedOrPromised(data.Name) {
		resp.Diagnostics.AddAttributeWarning(
			tfpath.Root("id"),
			"Missing Attribute Configuration",
			"Expected [id]",
		)
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
	resourceURL := path.Join(d.client.getEdaApiEndpoint(), d.ApiEntitySlug)
	params := map[string]string{
		"name": state.Name.ValueString(),
	}
	responseBody, diags := d.client.GetWithParams(resourceURL, params)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

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
		diags.AddError("Unable to fetch event_stream from AAP", fmt.Sprintf("Expected 1 object in JSON response, found %d", len(apiModelList.Results)))
		return diags
	}

	var apiModel = apiModelList.Results[0]

	d.ID = types.Int64Value(apiModel.Id)
	d.URL = ParseStringValue(apiModel.URL)
	d.Name = ParseStringValue(apiModel.Name)
	return diags
}
