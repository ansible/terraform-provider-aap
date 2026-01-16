package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"path"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure the implementation satisfies the desired interfaces.
var _ datasource.DataSource = &EDAProjectsDataSource{}
var _ datasource.DataSourceWithConfigure = &EDAProjectsDataSource{}

type EDAProjectsDataSource struct {
	client ProviderHTTPClient
}

type EDAProjectsDataSourceModel struct {
	OrganizationID types.Int64                   `tfsdk:"organization_id"`
	NameContains   types.String                  `tfsdk:"name_contains"`
	Projects       []EDAProjectDataSourceProject `tfsdk:"projects"`
}

type EDAProjectDataSourceProject struct {
	ID             types.Int64  `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Description    types.String `tfsdk:"description"`
	URL            types.String `tfsdk:"url"`
	SCMBranch      types.String `tfsdk:"scm_branch"`
	OrganizationID types.Int64  `tfsdk:"organization_id"`
	Proxy          types.String `tfsdk:"proxy"`
}

func NewEDAProjectsDataSource() datasource.DataSource {
	return &EDAProjectsDataSource{}
}

func (d *EDAProjectsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_eda_projects"
}

func (d *EDAProjectsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Gets a list of EDA Projects with optional filtering.",
		Attributes: map[string]schema.Attribute{
			"organization_id": schema.Int64Attribute{
				Optional:    true,
				Description: "Filter projects by organization ID.",
			},
			"name_contains": schema.StringAttribute{
				Optional:    true,
				Description: "Filter projects by name containing this string (case-insensitive).",
			},
			"projects": schema.ListNestedAttribute{
				Computed:    true,
				Description: "List of EDA projects matching the filters.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.Int64Attribute{
							Computed:    true,
							Description: "The ID of the EDA project.",
						},
						"name": schema.StringAttribute{
							Computed:    true,
							Description: "The name of the EDA project.",
						},
						"description": schema.StringAttribute{
							Computed:    true,
							Description: "The description of the EDA project.",
						},
						"url": schema.StringAttribute{
							Computed:    true,
							Description: "The SCM URL for the EDA project.",
						},
						"scm_branch": schema.StringAttribute{
							Computed:    true,
							Description: "The SCM branch for the EDA project.",
						},
						"organization_id": schema.Int64Attribute{
							Computed:    true,
							Description: "The organization ID for the EDA project.",
						},
						"proxy": schema.StringAttribute{
							Computed:    true,
							Description: "The proxy server for the EDA project.",
						},
					},
				},
			},
		},
	}
}

func (d *EDAProjectsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if resp == nil {
		tflog.Error(ctx, "Response not defined, we cannot continue with the execution")
		return
	}

	if !IsContextActive(ctx, "Configure", &resp.Diagnostics) {
		return
	}

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

func (d *EDAProjectsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state EDAProjectsDataSourceModel

	// Check Read preconditions
	if !DoReadPreconditionsMeet(ctx, resp, d.client) {
		return
	}

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build query parameters
	edaEndpoint := d.client.getEdaAPIEndpoint()
	if edaEndpoint == "" {
		resp.Diagnostics.AddError(
			"EDA API Endpoint is empty",
			"Expected a valid endpoint but was an empty string. Please report this issue to the provider developers.",
		)
		return
	}

	projectsURL := path.Join(edaEndpoint, "projects")
	params := make(map[string]string)

	if !state.OrganizationID.IsNull() {
		params["organization_id"] = fmt.Sprintf("%d", state.OrganizationID.ValueInt64())
	}

	if !state.NameContains.IsNull() && state.NameContains.ValueString() != "" {
		params["name__icontains"] = state.NameContains.ValueString()
	}

	// Fetch projects
	responseBody, diags := d.client.GetWithParams(projectsURL, params)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Parse response
	var listResponse EdaProjectListResponse
	err := json.Unmarshal(responseBody, &listResponse)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error parsing JSON response from AAP",
			fmt.Sprintf("Unable to parse EDA projects list response: %s", err.Error()),
		)
		return
	}

	// Convert API models to state models
	state.Projects = make([]EDAProjectDataSourceProject, len(listResponse.Results))
	for i, apiProject := range listResponse.Results {
		state.Projects[i] = EDAProjectDataSourceProject{
			ID:             types.Int64Value(apiProject.ID),
			Name:           types.StringValue(apiProject.Name),
			Description:    ParseStringValue(apiProject.Description),
			URL:            types.StringValue(apiProject.URL),
			SCMBranch:      ParseStringValue(apiProject.SCMBranch),
			OrganizationID: types.Int64Value(apiProject.OrganizationID),
			Proxy:          ParseStringValue(apiProject.Proxy),
		}
	}

	// Save updated state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
