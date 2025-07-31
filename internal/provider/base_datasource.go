package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"path"

	"github.com/ansible/terraform-provider-aap/internal/provider/customtypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	tfpath "github.com/hashicorp/terraform-plugin-framework/path"
	tftypes "github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource                     = &BaseDataSource{}
	_ datasource.DataSourceWithConfigure        = &BaseDataSource{}
	_ datasource.DataSourceWithConfigValidators = &BaseDataSource{}
	_ datasource.DataSourceWithValidateConfig   = &BaseDataSource{}

	_ datasource.DataSource                     = &BaseDataSourceWithOrg{}
	_ datasource.DataSourceWithConfigure        = &BaseDataSourceWithOrg{}
	_ datasource.DataSourceWithConfigValidators = &BaseDataSourceWithOrg{}
	_ datasource.DataSourceWithValidateConfig   = &BaseDataSourceWithOrg{}
)

// ---------------------------------------------------------------------------
// Constructors
// ---------------------------------------------------------------------------

// Constructs a new BaseDataSource object provided with a client instance (usually
// initialized to nil, it will be later configured calling the Configure function)
// and an apiEntitySlug string indicating the entity path name to consult the API.
func NewBaseDataSource(client ProviderHTTPClient, stringDescriptions StringDescriptions) *BaseDataSource {
	return &BaseDataSource{
		client:             client,
		StringDescriptions: stringDescriptions,
	}
}

// Constructs a new BaseDataSourceWithOrg object provided with a client instance (usually
// initialized to nil, it will be later configured calling the Configure function)
// and an apiEntitySlug string indicating the entity path name to consult the API.
func NewBaseDataSourceWithOrg(client ProviderHTTPClient, stringDescriptions StringDescriptions) *BaseDataSourceWithOrg {
	return &BaseDataSourceWithOrg{
		BaseDataSource: BaseDataSource{
			client:             client,
			StringDescriptions: stringDescriptions,
		},
	}
}

// ---------------------------------------------------------------------------
// Metadata
// ---------------------------------------------------------------------------

// Metadata returns the data source type name composing it from the provider type name and the
// entity slug string passed in the constructor.
func (d *BaseDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = fmt.Sprintf("%s_%s", req.ProviderTypeName, d.MetadataEntitySlug)
}

// ---------------------------------------------------------------------------
// Schema
// ---------------------------------------------------------------------------

// Schema defines the schema fields for the data source.
func (d *BaseDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Optional:    true,
				Description: fmt.Sprintf("%s id", d.DescriptiveEntityName),
			},
			"url": schema.StringAttribute{
				Computed:    true,
				Description: fmt.Sprintf("Url of the %s", d.DescriptiveEntityName),
			},
			"named_url": schema.StringAttribute{
				Computed:    true,
				Description: fmt.Sprintf("The Named Url of the %s", d.DescriptiveEntityName),
			},
			"name": schema.StringAttribute{
				Optional:    true,
				Description: fmt.Sprintf("Name of the %s", d.DescriptiveEntityName),
			},
			"description": schema.StringAttribute{
				Computed:    true,
				Description: fmt.Sprintf("Description of the %s", d.DescriptiveEntityName),
			},
			"variables": schema.StringAttribute{
				Computed:   true,
				CustomType: customtypes.AAPCustomStringType{},
				Description: fmt.Sprintf("Variables of the %s. Will be either JSON or YAML string depending on how the "+
					"variables were entered into AAP.", d.DescriptiveEntityName),
				DeprecationMessage: "This attribute is deprecated and will be removed in a future version.",
			},
		},
		Description: fmt.Sprintf("Get an existing %s.", d.DescriptiveEntityName),
	}
}

// Schema defines the schema fields for the data source.
func (d *BaseDataSourceWithOrg) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Optional:    true,
				Description: fmt.Sprintf("%s id", d.DescriptiveEntityName),
			},
			"organization": schema.Int64Attribute{
				Optional:    true,
				Description: fmt.Sprintf("Identifier for the organization to which the %s belongs", d.DescriptiveEntityName),
			},
			"organization_name": schema.StringAttribute{
				Optional:    true,
				Description: fmt.Sprintf("The name for the organization to which the %s belongs", d.DescriptiveEntityName),
			},
			"url": schema.StringAttribute{
				Computed:    true,
				Description: fmt.Sprintf("Url of the %s", d.DescriptiveEntityName),
			},
			"named_url": schema.StringAttribute{
				Computed:    true,
				Description: fmt.Sprintf("The Named Url of the %s", d.DescriptiveEntityName),
			},
			"name": schema.StringAttribute{
				Optional:    true,
				Description: fmt.Sprintf("Name of the %s", d.DescriptiveEntityName),
			},
			"description": schema.StringAttribute{
				Computed:    true,
				Description: fmt.Sprintf("Description of the %s", d.DescriptiveEntityName),
			},
			"variables": schema.StringAttribute{
				Computed:   true,
				CustomType: customtypes.AAPCustomStringType{},
				Description: fmt.Sprintf("Variables of the %s. Will be either JSON or YAML string depending on how the "+
					"variables were entered into AAP.", d.DescriptiveEntityName),
				DeprecationMessage: "This attribute is deprecated and will be removed in a future version.",
			},
		},
		Description: fmt.Sprintf("Get an existing %s.", d.DescriptiveEntityName),
	}
}

// ---------------------------------------------------------------------------
// ConfigValidators
// ---------------------------------------------------------------------------

func (d *BaseDataSource) ConfigValidators(_ context.Context) []datasource.ConfigValidator {
	// You have at least an id
	return []datasource.ConfigValidator{
		datasourcevalidator.Any(
			datasourcevalidator.AtLeastOneOf(
				tfpath.MatchRoot("id")),
		),
	}
}

func (d *BaseDataSourceWithOrg) ConfigValidators(_ context.Context) []datasource.ConfigValidator {
	// You have at least an id or a name + organization_name pair
	return []datasource.ConfigValidator{
		datasourcevalidator.Any(
			datasourcevalidator.AtLeastOneOf(
				tfpath.MatchRoot("id")),
			datasourcevalidator.RequiredTogether(
				tfpath.MatchRoot("name"),
				tfpath.MatchRoot("organization_name")),
		),
	}
}

// ---------------------------------------------------------------------------
// ValidateConfig
// ---------------------------------------------------------------------------

func (d *BaseDataSource) ValidateConfig(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	// Check that the response and diagnostics pointer is defined
	if resp == nil {
		tflog.Error(ctx, "Response not defined, we cannot continue with the execution")
		return
	}

	// Check that the current context is active
	if !IsContextActive("ValidateConfig", ctx, &resp.Diagnostics) {
		return
	}

	var data BaseDetailSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if IsValueProvidedOrPromised(data.Id) {
		return
	}

	if !IsValueProvidedOrPromised(data.Id) {
		resp.Diagnostics.AddAttributeWarning(
			tfpath.Root("id"),
			"Missing Attribute Configuration",
			"Expected [id]",
		)
	}
}

func (d *BaseDataSourceWithOrg) ValidateConfig(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	// Check that the response and diagnostics pointer is defined
	if resp == nil {
		tflog.Error(ctx, "Response not defined, we cannot continue with the execution")
		return
	}

	// Check that the current context is active
	if !IsContextActive("ValidateConfig", ctx, &resp.Diagnostics) {
		return
	}

	var data BaseDetailSourceModelWithOrg

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if IsValueProvidedOrPromised(data.Id) {
		return
	}

	if IsValueProvidedOrPromised(data.Name) && IsValueProvidedOrPromised(data.OrganizationName) {
		return
	}

	if !IsValueProvidedOrPromised(data.Id) && !IsValueProvidedOrPromised(data.Name) {
		resp.Diagnostics.AddAttributeWarning(
			tfpath.Root("id"),
			"Missing Attribute Configuration",
			"Expected either [id] or [name + organization_name] pair",
		)
	}

	if IsValueProvidedOrPromised(data.Name) && !IsValueProvidedOrPromised(data.OrganizationName) {
		resp.Diagnostics.AddAttributeWarning(
			tfpath.Root("organization_name"),
			"Missing Attribute Configuration",
			"Expected organization_name to be configured with name.",
		)
	}

	if !IsValueProvidedOrPromised(data.Name) && IsValueProvidedOrPromised(data.OrganizationName) {
		resp.Diagnostics.AddAttributeWarning(
			tfpath.Root("name"),
			"Missing Attribute Configuration",
			"Expected name to be configured with organization_name.",
		)
	}
}

// ---------------------------------------------------------------------------
// Configure
// ---------------------------------------------------------------------------

// Configure adds the provider configured client to the data source.
func (d *BaseDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

// ---------------------------------------------------------------------------
// Read
// ---------------------------------------------------------------------------

// Read refreshes the Terraform state with the latest data.
func (d *BaseDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state BaseDetailSourceModel
	var diags diag.Diagnostics

	// Check Read preconditions
	if !DoReadPreconditionsMeet(ctx, resp, d.client) {
		return
	}

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	uri := path.Join(d.client.getApiEndpoint(), d.ApiEntitySlug)
	resourceURL, err := state.CreateNamedURL(uri, &BaseDetailAPIModel{
		Id:   state.Id.ValueInt64(),
		Name: state.Name.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Minimal Data Not Supplied", "Expected [id]")
		return
	}

	readResponseBody, diags := d.client.Get(resourceURL)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = state.ParseHttpResponse(readResponseBody)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Set state
	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *BaseDataSourceWithOrg) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state BaseDetailSourceModelWithOrg
	var diags diag.Diagnostics

	// Check Read preconditions
	if !DoReadPreconditionsMeet(ctx, resp, d.client) {
		return
	}

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	uri := path.Join(d.client.getApiEndpoint(), d.ApiEntitySlug)
	resourceURL, err := state.CreateNamedURL(uri, &BaseDetailAPIModelWithOrg{
		BaseDetailAPIModel: BaseDetailAPIModel{
			Id:   state.Id.ValueInt64(),
			Name: state.Name.ValueString(),
		},
		SummaryFields: SummaryFieldsAPIModel{
			Organization: SummaryField{
				Id:   state.Organization.ValueInt64(),
				Name: state.OrganizationName.ValueString(),
			},
		},
	})
	if err != nil {
		resp.Diagnostics.AddError("Minimal Data Not Supplied", "Expected either [id] or [name + organization_name] pair")
		return
	}

	readResponseBody, diags := d.client.Get(resourceURL)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = state.ParseHttpResponse(readResponseBody)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Set state
	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// ---------------------------------------------------------------------------
// ParseHttpResponse
// ---------------------------------------------------------------------------

// This function allows us to parse the incoming data in HTTP requests from the API
// into the BaseDetailSourceModel instances.
func (d *BaseDetailSourceModel) ParseHttpResponse(body []byte) diag.Diagnostics {
	var diags diag.Diagnostics

	// Unmarshal the JSON response
	var apiModel BaseDetailAPIModel
	err := json.Unmarshal(body, &apiModel)
	if err != nil {
		diags.AddError("Error parsing JSON response from AAP", err.Error())
		return diags
	}

	// Map the response to the BaseDetailSourceModel datasource schema
	d.Id = tftypes.Int64Value(apiModel.Id)
	d.URL = ParseStringValue(apiModel.URL)
	d.Name = ParseStringValue(apiModel.Name)
	d.Description = ParseStringValue(apiModel.Description)
	d.Variables = ParseAAPCustomStringValue(apiModel.Variables)
	// Parse the related fields
	d.NamedUrl = ParseStringValue(apiModel.Related.NamedUrl)

	return diags
}

// This function allows us to parse the incoming data in HTTP requests from the API
// into the BaseDetailSourceModelWithOrg instances.
func (d *BaseDetailSourceModelWithOrg) ParseHttpResponse(body []byte) diag.Diagnostics {
	// Let my parent's ParseHttpResponse method handle the base fields
	diags := d.BaseDetailSourceModel.ParseHttpResponse(body)
	if diags.HasError() {
		return diags
	}

	// Unmarshal the JSON response
	var apiModel BaseDetailAPIModelWithOrg
	err := json.Unmarshal(body, &apiModel)
	if err != nil {
		diags.AddError("Error parsing JSON response from AAP", err.Error())
		return diags
	}

	// Map the response to the BaseDetailSourceModelWithOrg datasource schema
	d.Name = ParseStringValue(apiModel.Name)
	d.Description = ParseStringValue(apiModel.Description)
	d.Organization = tftypes.Int64Value(apiModel.Organization)
	d.Variables = ParseAAPCustomStringValue(apiModel.Variables)
	// Parse the related fields
	d.NamedUrl = ParseStringValue(apiModel.Related.NamedUrl)
	// Parse the summary fields
	d.OrganizationName = ParseStringValue(apiModel.SummaryFields.Organization.Name)

	return diags
}
