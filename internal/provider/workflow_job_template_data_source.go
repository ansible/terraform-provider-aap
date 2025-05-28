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

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource                     = &WorkflowJobTemplateDataSource{}
	_ datasource.DataSourceWithConfigure        = &WorkflowJobTemplateDataSource{}
	_ datasource.DataSourceWithConfigValidators = &WorkflowJobTemplateDataSource{}
	_ datasource.DataSourceWithValidateConfig   = &WorkflowJobTemplateDataSource{}
)

// NewWorkflowJobTemplateDataSource is a helper function to simplify the provider implementation.
func NewWorkflowJobTemplateDataSource() datasource.DataSource {
	return &WorkflowJobTemplateDataSource{}
}

// WorkflowJobTemplateDataSourceModel maps the data source schema data.
type WorkflowJobTemplateDataSourceModel struct {
	Id               types.Int64                      `tfsdk:"id"`
	Organization     types.Int64                      `tfsdk:"organization"`
	OrganizationName types.String                     `tfsdk:"organization_name"`
	Url              types.String                     `tfsdk:"url"`
	NamedUrl         types.String                     `tfsdk:"named_url"`
	Name             types.String                     `tfsdk:"name"`
	Description      types.String                     `tfsdk:"description"`
	Variables        customtypes.AAPCustomStringValue `tfsdk:"variables"`
}

// WorkflowJobTemplate AAP API model
type WorkflowJobTemplateAPIModel struct {
	Id            int64                 `json:"id,omitempty"`
	Organization  int64                 `json:"organization"`
	SummaryFields SummaryFieldsAPIModel `json:"summary_fields,omitempty"`
	Url           string                `json:"url,omitempty"`
	Related       RelatedAPIModel       `json:"related,omitempty"`
	Name          string                `json:"name"`
	Description   string                `json:"description,omitempty"`
	Variables     string                `json:"variables,omitempty"`
}

// WorkflowJobTemplateDataSource is the data source implementation.
type WorkflowJobTemplateDataSource struct {
	client ProviderHTTPClient
}

// Metadata returns the data source type name.
func (d *WorkflowJobTemplateDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workflow_job_template"
}

// Schema defines the schema for the data source.
func (d *WorkflowJobTemplateDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Optional:    true,
				Description: "WorkflowJobTemplate id",
			},
			"organization": schema.Int64Attribute{
				Computed:    true,
				Description: "Identifier for the organization to which the WorkflowJobTemplate belongs",
			},
			"organization_name": schema.StringAttribute{
				Computed:    true,
				Optional:    true,
				Description: "The name for the organization to which the WorkflowJobTemplate belongs",
			},
			"url": schema.StringAttribute{
				Computed:    true,
				Description: "Url of the WorkflowJobTemplate",
			},
			"named_url": schema.StringAttribute{
				Computed:    true,
				Description: "The Named Url of the WorkflowJobTemplate",
			},
			"name": schema.StringAttribute{
				Computed:    true,
				Optional:    true,
				Description: "Name of the WorkflowJobTemplate",
			},
			"description": schema.StringAttribute{
				Computed:    true,
				Description: "Description of the WorkflowJobTemplate",
			},
			"variables": schema.StringAttribute{
				Computed:    true,
				CustomType:  customtypes.AAPCustomStringType{},
				Description: "Variables of the WorkflowJobTemplate. Will be either JSON or YAML string depending on how the variables were entered into AAP.",
			},
		},
		Description: `Get an existing WorkflowJobTemplate.`,
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *WorkflowJobTemplateDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state WorkflowJobTemplateDataSourceModel
	var diags diag.Diagnostics

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	uri := path.Join(d.client.getApiEndpoint(), "workflow_job_templates")
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
func (d *WorkflowJobTemplateDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *WorkflowJobTemplateDataSource) ConfigValidators(_ context.Context) []datasource.ConfigValidator {
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

func (d *WorkflowJobTemplateDataSource) ValidateConfig(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	var data WorkflowJobTemplateDataSourceModel

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

func (d *WorkflowJobTemplateDataSourceModel) ParseHttpResponse(body []byte) diag.Diagnostics {
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
