package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/ansible/terraform-provider-aap/internal/provider/customtypes"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &WorkflowJobTemplateDataSource{}
	_ datasource.DataSourceWithConfigure = &WorkflowJobTemplateDataSource{}
)

// NewWorkflowJobTemplateDataSource is a helper function to simplify the provider implementation.
func NewWorkflowJobTemplateDataSource() datasource.DataSource {
	return &WorkflowJobTemplateDataSource{}
}

// WorkflowJobTemplateDataSource is the data source implementation.
type WorkflowJobTemplateDataSource struct {
	client *AAPClient
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
				Optional: true,
				Description: "WorkflowJobTemplate id",
			},
			"organization": schema.Int64Attribute{
				Computed: true,
				Description: "Identifier for the organization to which the WorkflowJobTemplate belongs",
			},
			"organization_name": schema.StringAttribute{
				Computed: true,
				Optional: true,
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
				Optional: true,
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

	//Here is where we can get the "named" WorkflowJobTemplate, which is "WorkflowJobTemplate Name"++"Organization Name" to derive uniqueness
	//we will take precedence if the Id is set to use that over the named_url attempt.

	resourceURL := ""

	if state.Id.String() != "<null>" {
		resourceURL = path.Join(d.client.getApiEndpoint(), "workflow_job_templates", state.Id.String())
	} else if state.Name.String() != "<null>" && state.OrganizationName.String() != "<null>"{
		namedUrl := strings.Join([]string{state.Name.String()[1 : len(state.Name.String()) - 1], "++", state.OrganizationName.String()[1 : len(state.OrganizationName.String()) - 1]}, "")
		resourceURL = path.Join(d.client.getApiEndpoint(), "workflow_job_templates", namedUrl)
	} else { 
		resp.Diagnostics.AddError("Minimal Data Not Supplied", "Require [id] or [name and organization_name]")
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

// WorkflowJobTemplateDataSourceModel maps the data source schema data.
type WorkflowJobTemplateDataSourceModel struct {
	Id           types.Int64                      `tfsdk:"id"`
	Organization types.Int64                      `tfsdk:"organization"`
	OrganizationName types.String                 `tfsdk:"organization_name"`
	Url          types.String                     `tfsdk:"url"`
	NamedUrl     types.String                     `tfsdk:"named_url"`
	Name         types.String                     `tfsdk:"name"`
	Description  types.String                     `tfsdk:"description"`
	Variables    customtypes.AAPCustomStringValue `tfsdk:"variables"`
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
	d.OrganizationName = types.StringValue(apiWorkflowJobTemplate.SummaryFields.Organization.Name)
	d.Url = types.StringValue(apiWorkflowJobTemplate.Url)
	d.NamedUrl = types.StringValue(apiWorkflowJobTemplate.Related.NamedUrl)
	d.Name = ParseStringValue(apiWorkflowJobTemplate.Name)
	d.Description = ParseStringValue(apiWorkflowJobTemplate.Description)
	d.Variables = ParseAAPCustomStringValue(apiWorkflowJobTemplate.Variables)

	return diags
}

// WorkflowJobTemplate AAP API model
type WorkflowJobTemplateAPIModel struct {
	Id           int64  `json:"id,omitempty"`
	Organization int64  `json:"organization"`
	SummaryFields SummaryFieldsAPIModel `json:"summary_fields,omitempty"`
	Url          string `json:"url,omitempty"`
	Related          RelatedAPIModel `json:"related,omitempty"`
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	Variables    string `json:"variables,omitempty"`
}