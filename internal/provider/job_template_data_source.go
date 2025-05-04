package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/ansible/terraform-provider-aap/internal/provider/customtypes"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// JobTemplate AAP API model
type JobTemplateAPIModel struct {
	Id            int64                 `json:"id,omitempty"`
	Organization  int64                 `json:"organization"`
	SummaryFields SummaryFieldsAPIModel `json:"summary_fields,omitempty"`
	Url           string                `json:"url,omitempty"`
	Related       RelatedAPIModel       `json:"related,omitempty"`
	Name          string                `json:"name"`
	Description   string                `json:"description,omitempty"`
	Variables     string                `json:"variables,omitempty"`
}

// JobTemplateDataSourceModel maps the data source schema data.
type JobTemplateDataSourceModel struct {
	Id               types.Int64                      `tfsdk:"id"`
	Organization     types.Int64                      `tfsdk:"organization"`
	OrganizationName types.String                     `tfsdk:"organization_name"`
	Url              types.String                     `tfsdk:"url"`
	NamedUrl         types.String                     `tfsdk:"named_url"`
	Name             types.String                     `tfsdk:"name"`
	Description      types.String                     `tfsdk:"description"`
	Variables        customtypes.AAPCustomStringValue `tfsdk:"variables"`
}

// JobTemplateDataSource is the data source implementation.
type JobTemplateDataSource struct {
	client ProviderHTTPClient
}

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &JobTemplateDataSource{}
	_ datasource.DataSourceWithConfigure = &JobTemplateDataSource{}
)

// NewJobTemplateDataSource is a helper function to simplify the provider implementation.
func NewJobTemplateDataSource() datasource.DataSource {
	return &JobTemplateDataSource{}
}

// Metadata returns the data source type name.
func (d *JobTemplateDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_job_template"
}

// Schema defines the schema for the data source.
func (d *JobTemplateDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Optional:    true,
				Description: "JobTemplate id",
			},
			"organization": schema.Int64Attribute{
				Computed:    true,
				Description: "Identifier for the organization to which the JobTemplate belongs",
			},
			"organization_name": schema.StringAttribute{
				Computed:    true,
				Optional:    true,
				Description: "The name for the organization to which the JobTemplate belongs",
			},
			"url": schema.StringAttribute{
				Computed:    true,
				Description: "Url of the JobTemplate",
			},
			"named_url": schema.StringAttribute{
				Computed:    true,
				Description: "The Named Url of the JobTemplate",
			},
			"name": schema.StringAttribute{
				Computed:    true,
				Optional:    true,
				Description: "Name of the JobTemplate",
			},
			"description": schema.StringAttribute{
				Computed:    true,
				Description: "Description of the JobTemplate",
			},
			"variables": schema.StringAttribute{
				Computed:    true,
				CustomType:  customtypes.AAPCustomStringType{},
				Description: "Variables of the JobTemplate. Will be either JSON or YAML string depending on how the variables were entered into AAP.",
			},
		},
		Description: `Get an existing JobTemplate.`,
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *JobTemplateDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state JobTemplateDataSourceModel
	var diags diag.Diagnostics

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resourceURL, err := state.ValidateLookupParameters(d)
	if err != nil {
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
func (d *JobTemplateDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (dm *JobTemplateDataSourceModel) ParseHttpResponse(body []byte) diag.Diagnostics {
	var diags diag.Diagnostics

	// Unmarshal the JSON response
	var apiJobTemplate JobTemplateAPIModel
	err := json.Unmarshal(body, &apiJobTemplate)
	if err != nil {
		diags.AddError("Error parsing JSON response from AAP", err.Error())
		return diags
	}

	// Map response to the JobTemplate datesource schema
	dm.Id = types.Int64Value(apiJobTemplate.Id)
	dm.Organization = types.Int64Value(apiJobTemplate.Organization)
	dm.OrganizationName = types.StringValue(apiJobTemplate.SummaryFields.Organization.Name)
	dm.Url = types.StringValue(apiJobTemplate.Url)
	dm.NamedUrl = types.StringValue(apiJobTemplate.Related.NamedUrl)
	dm.Name = ParseStringValue(apiJobTemplate.Name)
	dm.Description = ParseStringValue(apiJobTemplate.Description)
	dm.Variables = ParseAAPCustomStringValue(apiJobTemplate.Variables)

	return diags
}

// ValidateLookupParameters Validate the provided lookup parameters and return the appropriate resource url
func (dm *JobTemplateDataSourceModel) ValidateLookupParameters(datasource *JobTemplateDataSource) (string, error) {
	// Here is where we can get the "named" JobTemplate, which is "JobTemplate Name"++"Organization Name" to derive uniqueness
	// we will take precedence if the Id is set to use that over the named_url attempt.
	if dm.Id != types.Int64Null() {
		return path.Join(datasource.client.getApiEndpoint(), "job_templates", dm.Id.String()), nil
	} else if dm.Name != types.StringNull() && dm.OrganizationName != types.StringNull() {
		namedUrl := strings.Join([]string{dm.Name.String()[1 : len(dm.Name.String())-1], "++", dm.OrganizationName.String()[1 : len(dm.OrganizationName.String())-1]}, "")
		return path.Join(datasource.client.getApiEndpoint(), "job_templates", namedUrl), nil
	} else {
		return types.StringNull().String(), errors.New("invalid job lookup parameters")
	}
}
