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
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type SummaryFieldsAPIModel struct {
	Organization SummaryAPIModel `json:"organization,omitempty"`
	Inventory    SummaryAPIModel `json:"inventory,omitempty"`
}

// If we end up pulling in more summary_fields that have other information, we can split
// them out to their own structs at that time.
type SummaryAPIModel struct {
	Id          int64  `json:"id,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type RelatedAPIModel struct {
	NamedUrl string `json:"named_url,omitempty"`
}

// A base struct to represent the DataSource model so new Data Sources can
// extend it as needed.
type BaseDataSourceModel struct {
	Id               types.Int64                      `tfsdk:"id"`
	Name             types.String                     `tfsdk:"name"`
	Organization     types.Int64                      `tfsdk:"organization"`
	OrganizationName types.String                     `tfsdk:"organization_name"`
	Url              types.String                     `tfsdk:"url"`
	NamedUrl         types.String                     `tfsdk:"named_url"`
	Description      types.String                     `tfsdk:"description"`
	Variables        customtypes.AAPCustomStringValue `tfsdk:"variables"`
}

// This function allows us to parse the incoming data in HTTP requests from the API
// into the BaseDataSourceModel instances.
func (d *BaseDataSourceModel) ParseHttpResponse(body []byte) diag.Diagnostics {
	var diags diag.Diagnostics

	// Unmarshal the JSON response
	var apiWorkflowJobTemplate WorkflowJobTemplateAPIModel
	err := json.Unmarshal(body, &apiWorkflowJobTemplate)
	if err != nil {
		diags.AddError("Error parsing JSON response from AAP", err.Error())
		return diags
	}

	// Map response to the WorkflowJobTemplate datesource schema
	d.Id = types.Int64Value(apiWorkflowJobTemplate.Id)
	d.Organization = types.Int64Value(apiWorkflowJobTemplate.Organization)
	d.OrganizationName = ParseStringValue(apiWorkflowJobTemplate.SummaryFields.Organization.Name)
	d.Url = ParseStringValue(apiWorkflowJobTemplate.Url)
	d.NamedUrl = ParseStringValue(apiWorkflowJobTemplate.Related.NamedUrl)
	d.Name = ParseStringValue(apiWorkflowJobTemplate.Name)
	d.Description = ParseStringValue(apiWorkflowJobTemplate.Description)
	d.Variables = ParseAAPCustomStringValue(apiWorkflowJobTemplate.Variables)

	return diags
}

type StringDescriptions struct {
	ApiEntitySlug         string
	DescriptiveEntityName string
	MetadataEntitySlug    string
}

// A struct to represent a base DataSource object, with a client and the slug name of
// the API entity.
type BaseDataSource struct {
	client ProviderHTTPClient
	StringDescriptions
}

// Constructs a new BaseDataSource object provided with a client instance (usually
// initialized to nil, it will be later configured calling the Configure function)
// and an apiEntitySlug string indicating the entity path name to consult the API.
func NewBaseDataSource(client ProviderHTTPClient, stringDescriptions StringDescriptions) *BaseDataSource {
	return &BaseDataSource{
		client:             client,
		StringDescriptions: stringDescriptions,
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *BaseDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state BaseDataSourceModel
	var diags diag.Diagnostics

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	uri := path.Join(d.client.getApiEndpoint(), d.ApiEntitySlug)
	resourceURL, err := ReturnAAPNamedURL(state.Id, state.Name, state.OrganizationName, uri)
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

// Configure adds the provider configured client to the data source.
func (d *BaseDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *BaseDataSource) ConfigValidators(_ context.Context) []datasource.ConfigValidator {
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

func (d *BaseDataSource) ValidateConfig(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	var data BaseDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if IsValueProvided(data.Id) {
		return
	}

	if IsValueProvided(data.Name) && IsValueProvided(data.OrganizationName) {
		return
	}

	if !IsValueProvided(data.Id) && !IsValueProvided(data.Name) {
		resp.Diagnostics.AddAttributeWarning(
			tfpath.Root("id"),
			"Missing Attribute Configuration",
			"Expected either [id] or [name + organization_name] pair",
		)
	}

	if IsValueProvided(data.Name) && !IsValueProvided(data.OrganizationName) {
		resp.Diagnostics.AddAttributeWarning(
			tfpath.Root("organization_name"),
			"Missing Attribute Configuration",
			"Expected organization_name to be configured with name.",
		)
	}

	if !IsValueProvided(data.Name) && IsValueProvided(data.OrganizationName) {
		resp.Diagnostics.AddAttributeWarning(
			tfpath.Root("name"),
			"Missing Attribute Configuration",
			"Expected name to be configured with organization_name.",
		)
	}
}

// Schema defines the schema fields for the data source.
func (d *BaseDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Optional:    true,
				Description: fmt.Sprintf("%s id", d.DescriptiveEntityName),
			},
			"organization": schema.Int64Attribute{
				Computed:    true,
				Description: fmt.Sprintf("Identifier for the organization to which the %s belongs", d.DescriptiveEntityName),
			},
			"organization_name": schema.StringAttribute{
				Computed:    true,
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
				Computed:    true,
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
			},
		},
		Description: fmt.Sprintf("Get an existing %s.", d.DescriptiveEntityName),
	}
}

// Metadata returns the data source type name composing it from the provider type name and the
// entity slug string passed in the constructor.
func (d *BaseDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = fmt.Sprintf("%s_%s", req.ProviderTypeName, d.MetadataEntitySlug)
}
