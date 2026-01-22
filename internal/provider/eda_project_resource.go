package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource                = &EdaProjectResource{}
	_ resource.ResourceWithConfigure   = &EdaProjectResource{}
	_ resource.ResourceWithImportState = &EdaProjectResource{}
)

func NewEdaProjectResource() resource.Resource {
	return &EdaProjectResource{}
}

type EdaProjectResource struct {
	client ProviderHTTPClient
}

type EdaProjectResourceModel struct {
	ID             types.Int64  `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Description    types.String `tfsdk:"description"`
	URL            types.String `tfsdk:"url"`
	SCMBranch      types.String `tfsdk:"scm_branch"`
	OrganizationID types.Int64  `tfsdk:"organization_id"`
	Proxy          types.String `tfsdk:"proxy"`
}

type EdaProjectAPIModel struct {
	ID             int64  `json:"id,omitempty"`
	Name           string `json:"name"`
	Description    string `json:"description,omitempty"`
	URL            string `json:"url"`
	SCMBranch      string `json:"scm_branch,omitempty"`
	OrganizationID int64  `json:"organization_id"`
	Proxy          string `json:"proxy,omitempty"`
}

type EdaProjectListResponse struct {
	Count   int                  `json:"count"`
	Results []EdaProjectAPIModel `json:"results"`
}

func (r *EdaProjectResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_eda_project"
}

func (r *EdaProjectResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an EDA Project resource.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:    true,
				Description: "The ID of the EDA project.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The name of the EDA project.",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "The description of the EDA project.",
			},
			"url": schema.StringAttribute{
				Required:    true,
				Description: "The SCM URL for the EDA project.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"scm_branch": schema.StringAttribute{
				Optional:    true,
				Description: "The SCM branch for the EDA project.",
			},
			"organization_id": schema.Int64Attribute{
				Required:    true,
				Description: "The organization ID for the EDA project.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"proxy": schema.StringAttribute{
				Optional:    true,
				Description: "The proxy server for the EDA project.",
			},
		},
	}
}

func (r *EdaProjectResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if resp == nil {
		tflog.Error(ctx, "Response not defined, we cannot continue with the execution.")
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
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *AAPClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *EdaProjectResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan EdaProjectResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	requestBody, diags := plan.generateRequestBody()
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	edaEndpoint := r.client.getEdaAPIEndpoint()
	if edaEndpoint == "" {
		resp.Diagnostics.AddError(
			"EDA API Endpoint is empty",
			"Expected a valid endpoint but was an empty string. Please report this issue to the provider developers.",
		)
		return
	}

	projectsURL := path.Join(edaEndpoint, "projects")
	requestData := bytes.NewReader(requestBody)
	createResponseBody, diags := r.client.Create(projectsURL, requestData)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = plan.parseHTTPResponse(createResponseBody)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *EdaProjectResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state EdaProjectResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	edaEndpoint := r.client.getEdaAPIEndpoint()
	if edaEndpoint == "" {
		resp.Diagnostics.AddError(
			"EDA API Endpoint is empty",
			"Expected a valid endpoint but was an empty string. Please report this issue to the provider developers.",
		)
		return
	}

	projectsURL := path.Join(edaEndpoint, "projects")
	params := map[string]string{
		"name": state.Name.ValueString(),
	}

	readResponseBody, diags := r.client.GetWithParams(projectsURL, params)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var listResponse EdaProjectListResponse
	err := json.Unmarshal(readResponseBody, &listResponse)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error parsing JSON response from AAP",
			fmt.Sprintf("Unable to parse EDA project list response: %s", err.Error()),
		)
		return
	}

	if listResponse.Count == 0 {
		resp.State.RemoveResource(ctx)
		return
	}

	if listResponse.Count > 1 {
		resp.Diagnostics.AddError(
			"Multiple EDA Projects found",
			fmt.Sprintf("Expected 1 project with name %s, found %d", state.Name.ValueString(), listResponse.Count),
		)
		return
	}

	project := listResponse.Results[0]
	diags = state.parseAPIModel(&project)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *EdaProjectResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan EdaProjectResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	requestBody, diags := plan.generateRequestBody()
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	edaEndpoint := r.client.getEdaAPIEndpoint()
	if edaEndpoint == "" {
		resp.Diagnostics.AddError(
			"EDA API Endpoint is empty",
			"Expected a valid endpoint but was an empty string. Please report this issue to the provider developers.",
		)
		return
	}

	projectURL := path.Join(edaEndpoint, "projects", strconv.FormatInt(plan.ID.ValueInt64(), 10))
	requestData := bytes.NewReader(requestBody)
	updateResponseBody, diags := r.client.Patch(projectURL, requestData)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = plan.parseHTTPResponse(updateResponseBody)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *EdaProjectResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id := req.ID

	var projectID int64
	_, err := fmt.Sscanf(id, "%d", &projectID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Expected numeric project ID, got: %s", id),
		)
		return
	}

	var state EdaProjectResourceModel
	state.ID = types.Int64Value(projectID)

	edaEndpoint := r.client.getEdaAPIEndpoint()
	if edaEndpoint == "" {
		resp.Diagnostics.AddError(
			"EDA API Endpoint is empty",
			"Expected a valid endpoint but was an empty string. Please report this issue to the provider developers.",
		)
		return
	}

	projectURL := path.Join(edaEndpoint, "projects", strconv.FormatInt(projectID, 10))
	readResponseBody, diags := r.client.Get(projectURL)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = state.parseHTTPResponse(readResponseBody)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *EdaProjectResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state EdaProjectResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	edaEndpoint := r.client.getEdaAPIEndpoint()
	if edaEndpoint == "" {
		resp.Diagnostics.AddError(
			"EDA API Endpoint is empty",
			"Expected a valid endpoint but was an empty string. Please report this issue to the provider developers.",
		)
		return
	}

	projectURL := path.Join(edaEndpoint, "projects", strconv.FormatInt(state.ID.ValueInt64(), 10))
	_, diags, statusCode := r.client.DeleteWithStatus(projectURL)
	if statusCode == http.StatusNotFound {
		return
	}
	resp.Diagnostics.Append(diags...)
}

func (r *EdaProjectResourceModel) generateRequestBody() ([]byte, diag.Diagnostics) {
	project := EdaProjectAPIModel{
		Name:           r.Name.ValueString(),
		Description:    r.Description.ValueString(),
		URL:            r.URL.ValueString(),
		SCMBranch:      r.SCMBranch.ValueString(),
		OrganizationID: r.OrganizationID.ValueInt64(),
		Proxy:          r.Proxy.ValueString(),
	}

	jsonBody, err := json.Marshal(project)
	if err != nil {
		var diags diag.Diagnostics
		diags.AddError(
			"Error marshaling request body",
			fmt.Sprintf("Could not generate request body for EDA project resource, unexpected error: %s", err.Error()),
		)
		return nil, diags
	}

	return jsonBody, nil
}

func (r *EdaProjectResourceModel) parseHTTPResponse(body []byte) diag.Diagnostics {
	var apiProject EdaProjectAPIModel
	err := json.Unmarshal(body, &apiProject)
	if err != nil {
		var diags diag.Diagnostics
		diags.AddError("Error parsing JSON response from AAP", err.Error())
		return diags
	}

	return r.parseAPIModel(&apiProject)
}

func (r *EdaProjectResourceModel) parseAPIModel(apiProject *EdaProjectAPIModel) diag.Diagnostics {
	r.ID = types.Int64Value(apiProject.ID)
	r.Name = types.StringValue(apiProject.Name)
	r.Description = ParseStringValue(apiProject.Description)
	r.URL = types.StringValue(apiProject.URL)
	r.SCMBranch = ParseStringValue(apiProject.SCMBranch)
	r.OrganizationID = types.Int64Value(apiProject.OrganizationID)
	r.Proxy = ParseStringValue(apiProject.Proxy)

	return nil
}
