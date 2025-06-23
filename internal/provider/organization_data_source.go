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

// Organization AAP API model
type OrganizationAPIModel struct {
	Id            int64                 `json:"id,omitempty"`
	SummaryFields SummaryFieldsAPIModel `json:"summary_fields,omitempty"`
	Url           string                `json:"url,omitempty"`
	Related       RelatedAPIModel       `json:"related,omitempty"`
	Name          string                `json:"name"`
	Description   string                `json:"description,omitempty"`
	Variables     string                `json:"variables,omitempty"`
}

// organizationDataSourceModel maps the data source schema data.
type OrganizationDataSourceModel struct {
	Id          types.Int64                      `tfsdk:"id"`
	Url         types.String                     `tfsdk:"url"`
	NamedUrl    types.String                     `tfsdk:"named_url"`
	Name        types.String                     `tfsdk:"name"`
	Description types.String                     `tfsdk:"description"`
	Variables   customtypes.AAPCustomStringValue `tfsdk:"variables"`
}

// OrganizationDataSource is the data source implementation.
type OrganizationDataSource struct {
	client ProviderHTTPClient
}

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource                     = &OrganizationDataSource{}
	_ datasource.DataSourceWithConfigure        = &OrganizationDataSource{}
	_ datasource.DataSourceWithConfigValidators = &OrganizationDataSource{}
	_ datasource.DataSourceWithValidateConfig   = &OrganizationDataSource{}
)

// NewOrganizationDataSource is a helper function to simplify the provider implementation.
func NewOrganizationDataSource() datasource.DataSource {
	return &OrganizationDataSource{}
}

// Metadata returns the data source type name.
func (d *OrganizationDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization"
}

// Schema defines the schema for the data source.
func (d *OrganizationDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Optional:    true,
				Description: "Organization id",
			},
			"name": schema.StringAttribute{
				Computed:    true,
				Optional:    true,
				Description: "Name of the organization",
			},
			"url": schema.StringAttribute{
				Computed:    true,
				Description: "Url of the organization",
			},
			"named_url": schema.StringAttribute{
				Computed:    true,
				Description: "The Named Url of the organization",
			},
			"description": schema.StringAttribute{
				Computed:    true,
				Description: "Description of the organization",
			},
			"variables": schema.StringAttribute{
				Computed:    true,
				CustomType:  customtypes.AAPCustomStringType{},
				Description: "Variables of the organization. Will be either JSON or YAML string depending on how the variables were entered into AAP.",
			},
		},
		Description: `Get an existing organization.`,
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *OrganizationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state OrganizationDataSourceModel
	var diags diag.Diagnostics

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	uri := path.Join(d.client.getApiEndpoint(), "organizations")
	resourceURL, err := ReturnAAPOrganizationNamedURL(state.Id, state.Name, uri)
	if err != nil {
		resp.Diagnostics.AddError("Minimal Data Not Supplied", "Expected either [id] or [name]")
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
func (d *OrganizationDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *OrganizationDataSource) ConfigValidators(_ context.Context) []datasource.ConfigValidator {
	// You have at least an id or a name + organization_name pair
	return []datasource.ConfigValidator{
		datasourcevalidator.AtLeastOneOf(
			tfpath.MatchRoot("id"),
			tfpath.MatchRoot("name"),
		),
	}
}

func (d *OrganizationDataSource) ValidateConfig(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	var data OrganizationDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if IsValueProvided(data.Id) && IsValueProvided(data.Name) {
		resp.Diagnostics.AddAttributeError(
			tfpath.Root("id"),
			"Attribute Precedence",
			"When both [id] and [name] are defined for aap_organization, id takes precedence.",
		)
		resp.Diagnostics.AddAttributeWarning(
			tfpath.Root("name"),
			"Attribute Precedence",
			"When both [id] and [name] are defined for aap_organization, id takes precedence.",
		)
	}

	if IsValueProvided(data.Id) {
		return
	}

	if IsValueProvided(data.Name) {
		return
	}

	if !IsValueProvided(data.Id) && !IsValueProvided(data.Name) {
		resp.Diagnostics.AddAttributeWarning(
			tfpath.Root("id"),
			"Missing Attribute Configuration",
			"Expected either [id] or [name]",
		)
		resp.Diagnostics.AddAttributeWarning(
			tfpath.Root("name"),
			"Missing Attribute Configuration",
			"Expected either [id] or [name]",
		)
	}
}

func (dm *OrganizationDataSourceModel) ParseHttpResponse(body []byte) diag.Diagnostics {
	var diags diag.Diagnostics

	// Unmarshal the JSON response
	var apiOrganization OrganizationAPIModel
	err := json.Unmarshal(body, &apiOrganization)
	if err != nil {
		diags.AddError("Error parsing JSON response from AAP", err.Error())
		return diags
	}

	// Map response to the organization datesource schema
	dm.Id = types.Int64Value(apiOrganization.Id)
	dm.Name = ParseStringValue(apiOrganization.Name)
	dm.Url = types.StringValue(apiOrganization.Url)
	dm.NamedUrl = types.StringValue(apiOrganization.Related.NamedUrl)
	dm.Description = ParseStringValue(apiOrganization.Description)
	dm.Variables = ParseAAPCustomStringValue(apiOrganization.Variables)

	return diags
}
