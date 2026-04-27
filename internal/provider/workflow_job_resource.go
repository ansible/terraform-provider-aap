package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ansible/terraform-provider-aap/internal/provider/customtypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
)

// WorkflowJobAPIModel represents the AAP API model for workflow jobs.
// /api/controller/v2/workflow_jobs/<id>/
type WorkflowJobAPIModel struct {
	TemplateID    int64                  `json:"workflow_job_template,omitempty"`
	Inventory     int64                  `json:"inventory,omitempty"`
	Type          string                 `json:"job_type,omitempty"`
	URL           string                 `json:"url,omitempty"`
	Status        string                 `json:"status,omitempty"`
	ExtraVars     string                 `json:"extra_vars,omitempty"`
	Limit         string                 `json:"limit,omitempty"`
	JobTags       string                 `json:"job_tags,omitempty"`
	SkipTags      string                 `json:"skip_tags,omitempty"`
	IgnoredFields map[string]interface{} `json:"ignored_fields,omitempty"`
}

// WorkflowJobLaunchAPIModel represents the AAP API model for Workflow Job Template launch endpoint.
// GET /api/controller/v2/workflow_job_templates/<id>/launch/
// It helps determine if a workflow_job_template can be launched.
type WorkflowJobLaunchAPIModel struct {
	AskVariablesOnLaunch    bool     `json:"ask_variables_on_launch"`
	AskTagsOnLaunch         bool     `json:"ask_tags_on_launch"`
	AskSkipTagsOnLaunch     bool     `json:"ask_skip_tags_on_launch"`
	AskLimitOnLaunch        bool     `json:"ask_limit_on_launch"`
	AskInventoryOnLaunch    bool     `json:"ask_inventory_on_launch"`
	AskLabelsOnLaunch       bool     `json:"ask_labels_on_launch"`
	SurveyEnabled           bool     `json:"survey_enabled"`
	VariablesNeededToStart  []string `json:"variables_needed_to_start"`
	InventoryNeededToStart  bool     `json:"inventory_needed_to_start"`
	CredentialNeededToStart bool     `json:"credential_needed_to_start"`
}

// WorkflowJobLaunchRequestModel represents the request body for POST /workflow_job_templates/{id}/launch.
// This is separate from WorkflowJobAPIModel because the POST request has different field formats.
// Note: labels is sent as an array of integer IDs [N, M].
type WorkflowJobLaunchRequestModel struct {
	Inventory int64   `json:"inventory,omitempty"`
	ExtraVars string  `json:"extra_vars,omitempty"`
	Limit     string  `json:"limit,omitempty"`
	JobTags   string  `json:"job_tags,omitempty"`
	SkipTags  string  `json:"skip_tags,omitempty"`
	Labels    []int64 `json:"labels,omitempty"`
}

// WorkflowJobModel are the attributes that are provided by the user and also used by the action.
type WorkflowJobModel struct {
	TemplateID               types.Int64                      `tfsdk:"workflow_job_template_id"`
	InventoryID              types.Int64                      `tfsdk:"inventory_id"`
	ExtraVars                customtypes.AAPCustomStringValue `tfsdk:"extra_vars"`
	WaitForCompletion        types.Bool                       `tfsdk:"wait_for_completion"`
	WaitForCompletionTimeout types.Int64                      `tfsdk:"wait_for_completion_timeout_seconds"`
	Limit                    customtypes.AAPCustomStringValue `tfsdk:"limit"`
	JobTags                  customtypes.AAPCustomStringValue `tfsdk:"job_tags"`
	SkipTags                 customtypes.AAPCustomStringValue `tfsdk:"skip_tags"`
	Labels                   types.List                       `tfsdk:"labels"`
}

// WorkflowJobResourceModel maps the resource schema data.
type WorkflowJobResourceModel struct {
	WorkflowJobModel
	Type          types.String `tfsdk:"job_type"`
	URL           types.String `tfsdk:"url"`
	Status        types.String `tfsdk:"status"`
	IgnoredFields types.List   `tfsdk:"ignored_fields"`
	Triggers      types.Map    `tfsdk:"triggers"`
}

// WorkflowJobResource is the resource implementation.
type WorkflowJobResource struct {
	client ProviderHTTPClient
}

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource              = &WorkflowJobResource{}
	_ resource.ResourceWithConfigure = &WorkflowJobResource{}
)

// NewWorkflowJobResource is a helper function to simplify the provider implementation.
func NewWorkflowJobResource() resource.Resource {
	return &WorkflowJobResource{}
}

// Metadata returns the resource type name.
func (r *WorkflowJobResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workflow_job"
}

// Configure adds the provider configured client to the data source.
func (r *WorkflowJobResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
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

// Schema defines the schema for the  workflowjobresource.
func (r *WorkflowJobResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	// Start with common attributes
	attributes := CommonJobSchemaAttributes("workflow job")

	// Add workflow-specific attributes
	attributes["inventory_id"] = schema.Int64Attribute{
		Optional: true,
		Computed: true,
		PlanModifiers: []planmodifier.Int64{
			int64planmodifier.UseStateForUnknown(),
		},
		Description: "Identifier for the inventory the job will be run against.",
	}
	attributes["workflow_job_template_id"] = schema.Int64Attribute{
		Required:    true,
		Description: "ID of the workflow job template.",
	}

	resp.Schema = schema.Schema{
		Attributes: attributes,
		MarkdownDescription: "Launches an AAP workflow job.\n\n" +
			"A workflow job is launched only when the resource is first created or when the " +
			"resource is changed. The " + "`triggers`" + " argument can be used to " +
			"launch a new workflow job based on any arbitrary value.\n\n" +
			"This resource always creates a new workflow job in AAP. A destroy will not " +
			"delete a workflow job created by this resource, it will only remove the resource " +
			"from the state.",
	}
}

// Create creates a new workflow job resource.
func (r *WorkflowJobResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data WorkflowJobResourceModel

	// Read Terraform plan data into workflow job resource model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// WriteOnly attributes (labels) must be read from the config,
	// not the plan, because WriteOnly values are always null in the plan.
	var configData WorkflowJobResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &configData)...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Labels = configData.Labels

	// Launch job and wait for completion if configured
	resp.Diagnostics.Append(r.launchAndWait(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *WorkflowJobResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data WorkflowJobResourceModel
	var diags diag.Diagnostics

	// Read current Terraform state data into job resource model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get latest workflow job data from AAP
	readResponseBody, diags, status := r.client.GetWithStatus(data.URL.ValueString(), nil)

	// Check if the response is 404, meaning the job does not exist and should be recreated
	if status == http.StatusNotFound {
		resp.Diagnostics.AddWarning(
			"Workflow job not found",
			"The workflow job was not found. It may have been deleted. The workflow job will be recreated.",
		)
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save latest workflow job data into workflow job resource model
	diags = data.ParseHTTPResponse(readResponseBody)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update updates an existing workflow job resource.
func (r *WorkflowJobResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data WorkflowJobResourceModel

	// Read Terraform plan data into workflow job resource model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// WriteOnly attributes (labels) must be read from the config,
	// not the plan, because WriteOnly values are always null in the plan.
	var configData WorkflowJobResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &configData)...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Labels = configData.Labels

	// Launch job and wait for completion if configured
	resp.Diagnostics.Append(r.launchAndWait(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete is intentionally left blank Job and Workflow Job Resources.
// Current guidance is to manage this inside AAP.
func (r WorkflowJobResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

// launchAndWait launches a workflow job and optionally waits for completion.
// This is shared logic between Create and Update operations.
func (r *WorkflowJobResource) launchAndWait(
	ctx context.Context,
	data *WorkflowJobResourceModel,
) diag.Diagnostics {
	var diags diag.Diagnostics

	// Launch the workflow job
	diags.Append(data.LaunchWorkflowJobWithResponse(r.client)...)
	if diags.HasError() {
		return diags
	}

	// Wait for completion if configured
	if data.WaitForCompletion.ValueBool() {
		retryProgressFunc := func(status string) {
			tflog.Debug(ctx, "Job status update", map[string]interface{}{
				"status": status,
				"url":    data.URL.ValueString(),
			})
		}
		status, waitDiags := data.WaitForWorkflowJobCompletion(ctx, r.client, data.URL.ValueString(), retryProgressFunc)
		diags.Append(waitDiags...)
		if diags.HasError() {
			return diags
		}
		data.Status = types.StringValue(status)
	}

	return diags
}

// CreateRequestBody creates a JSON encoded request body from the workflow job resource data.
// Null/unknown fields return zero values which are omitted via omitempty JSON tags.
func (r *WorkflowJobModel) CreateRequestBody() ([]byte, diag.Diagnostics) {
	var diags diag.Diagnostics

	req := WorkflowJobLaunchRequestModel{
		ExtraVars: r.ExtraVars.ValueString(),
		Limit:     r.Limit.ValueString(),
		JobTags:   r.JobTags.ValueString(),
		SkipTags:  r.SkipTags.ValueString(),
		Inventory: r.InventoryID.ValueInt64(),
		Labels:    ConvertListToInt64Slice(r.Labels),
	}

	jsonBody, err := json.Marshal(req)
	if err != nil {
		diags.AddError(
			"Error marshaling request body",
			fmt.Sprintf("Could not create request body for workflow job resource, unexpected error: %s", err.Error()),
		)
		return nil, diags
	}
	return jsonBody, diags
}

// ParseHTTPResponse updates the workflow job resource data from an AAP API response.
func (r *WorkflowJobResourceModel) ParseHTTPResponse(body []byte) diag.Diagnostics {
	var diags diag.Diagnostics

	// Unmarshal the JSON response
	var resultAPIWorkflowJob WorkflowJobAPIModel
	err := json.Unmarshal(body, &resultAPIWorkflowJob)
	if err != nil {
		diags.AddError("Error parsing JSON response from AAP", err.Error())
		return diags
	}

	// Map response to the job resource schema and update attribute values.
	// All Optional+Computed fields use UseStateForUnknown() plan modifiers,
	// so we can safely set values from the API response without causing drift.
	r.Type = types.StringValue(resultAPIWorkflowJob.Type)
	r.URL = types.StringValue(resultAPIWorkflowJob.URL)
	r.Status = types.StringValue(resultAPIWorkflowJob.Status)
	r.TemplateID = types.Int64Value(resultAPIWorkflowJob.TemplateID)
	r.InventoryID = types.Int64Value(resultAPIWorkflowJob.Inventory)
	r.Limit = customtypes.NewAAPCustomStringValue(resultAPIWorkflowJob.Limit)
	r.JobTags = customtypes.NewAAPCustomStringValue(resultAPIWorkflowJob.JobTags)
	r.SkipTags = customtypes.NewAAPCustomStringValue(resultAPIWorkflowJob.SkipTags)

	// Labels are WriteOnly and handled separately via API

	diags = r.ParseIgnoredFields(resultAPIWorkflowJob.IgnoredFields)
	return diags
}

// ParseIgnoredFields parses ignored fields from the AAP API response.
func (r *WorkflowJobResourceModel) ParseIgnoredFields(ignoredFields map[string]interface{}) (diags diag.Diagnostics) {
	r.IgnoredFields, diags = ParseIgnoredFieldsToList(ignoredFields)
	return diags
}

// GetLaunchWorkflowJob performs a GET request to the Workflow Job Template launch endpoint to retrieve
// the launch configuration.
func (r *WorkflowJobModel) GetLaunchWorkflowJob(client ProviderHTTPClient) (launchConfig WorkflowJobLaunchAPIModel, diags diag.Diagnostics) {
	diags = GetLaunchConfiguration(client, "workflow_job_templates", r.TemplateID.ValueInt64(), &launchConfig, "Workflow Job Template")
	return launchConfig, diags
}

// CanWorkflowJobBeLaunched retrieves the launch configuration and validates that all required
// fields are provided. It also warns when fields are provided but will be ignored.
// This determines if a Workflow Job Template can be launched.
func (r *WorkflowJobModel) CanWorkflowJobBeLaunched(client ProviderHTTPClient) (diags diag.Diagnostics) {
	launchConfig, diags := r.GetLaunchWorkflowJob(client)
	if diags.HasError() {
		return diags
	}

	validations := []LaunchFieldValidation{
		{launchConfig.AskVariablesOnLaunch, r.ExtraVars, "extra_vars"},
		{launchConfig.AskTagsOnLaunch, r.JobTags, "job_tags"},
		{launchConfig.AskSkipTagsOnLaunch, r.SkipTags, "skip_tags"},
		{launchConfig.AskLimitOnLaunch, r.Limit, "limit"},
		{launchConfig.AskInventoryOnLaunch, r.InventoryID, "inventory_id"},
		{launchConfig.AskLabelsOnLaunch, r.Labels, "labels"},
	}

	requirements := LaunchRequirements{
		VariablesNeededToStart:  launchConfig.VariablesNeededToStart,
		InventoryNeededToStart:  launchConfig.InventoryNeededToStart,
		CredentialNeededToStart: launchConfig.CredentialNeededToStart,
	}

	diags.Append(ValidateLaunchFields(requirements, validations, "Workflow Job Template", r.ExtraVars)...)
	return diags
}

// LaunchWorkflowJob launches a workflow job from the Workflow Job Template.
// It first checks if the workflow job can be launched, then POSTs to launch the job.
func (r *WorkflowJobModel) LaunchWorkflowJob(client ProviderHTTPClient) ([]byte, diag.Diagnostics) {
	// First, check if the workflow job can be launched
	diags := r.CanWorkflowJobBeLaunched(client)
	if diags.HasError() {
		return nil, diags
	}

	// Use shared launch helper
	return LaunchJobTemplate(client, "workflow_job_templates", r)
}

func (r *WorkflowJobResourceModel) LaunchWorkflowJobWithResponse(client ProviderHTTPClient) diag.Diagnostics {
	body, diags := r.LaunchWorkflowJob(client)
	if diags.HasError() {
		return diags
	}
	return r.ParseHTTPResponse(body)
}

// WaitForWorkflowJobCompletion waits for a workflow job to reach a final state.
// It returns the final status and any diagnostics.
func (r *WorkflowJobModel) WaitForWorkflowJobCompletion(
	ctx context.Context,
	client ProviderHTTPClient,
	jobURL string,
	retryProgressFunc RetryProgressFunc,
) (status string, diags diag.Diagnostics) {
	timeout := time.Duration(r.WaitForCompletionTimeout.ValueInt64()) * time.Second
	err := retry.RetryContext(ctx, timeout, retryUntilAAPJobReachesAnyFinalState(ctx, client, retryProgressFunc, jobURL, &status))
	if err != nil {
		diags.AddError("error when waiting for AAP Workflow job to complete", err.Error())
		return status, diags
	}
	return status, diags
}

// GetTemplateID implements the LaunchableJob interface.
func (r *WorkflowJobModel) GetTemplateID() int64 {
	return r.TemplateID.ValueInt64()
}
