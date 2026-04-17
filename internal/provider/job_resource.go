package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ansible/terraform-provider-aap/internal/provider/customtypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
)

const (
	// Default value for the wait_for_completion timeout, so the linter doesn't complain.
	waitForCompletionTimeoutDefault int64  = 120
	statusSuccessfulConst           string = "successful"
	// VerbosityMax is the maximum verbosity level for job runs (WinRM Debug).
	VerbosityMax int64 = 5
)

// JobAPIModel represents the AAP API model. /api/controller/v2/jobs/<id>/
type JobAPIModel struct {
	TemplateID           int64                  `json:"job_template,omitempty"`
	Type                 string                 `json:"job_type,omitempty"`
	URL                  string                 `json:"url,omitempty"`
	Status               string                 `json:"status,omitempty"`
	Inventory            int64                  `json:"inventory,omitempty"`
	ExtraVars            string                 `json:"extra_vars,omitempty"`
	IgnoredFields        map[string]interface{} `json:"ignored_fields,omitempty"`
	Limit                string                 `json:"limit,omitempty"`
	JobTags              string                 `json:"job_tags,omitempty"`
	SkipTags             string                 `json:"skip_tags,omitempty"`
	DiffMode             bool                   `json:"diff_mode,omitempty"`
	Verbosity            int64                  `json:"verbosity,omitempty"`
	ExecutionEnvironment int64                  `json:"execution_environment,omitempty"`
	Forks                int64                  `json:"forks,omitempty"`
	JobSliceCount        int64                  `json:"job_slice_count,omitempty"`
	Timeout              int64                  `json:"timeout,omitempty"`
	InstanceGroup        int64                  `json:"instance_group,omitempty"`
}

// JobLaunchAPIModel represents the AAP API model for Job Template launch endpoint.
// GET /api/controller/v2/job_templates/<id>/launch/
// It helps determine if a job_template can be launched.
type JobLaunchAPIModel struct {
	AskVariablesOnLaunch            bool `json:"ask_variables_on_launch"`
	AskTagsOnLaunch                 bool `json:"ask_tags_on_launch"`
	AskSkipTagsOnLaunch             bool `json:"ask_skip_tags_on_launch"`
	AskJobTypeOnLaunch              bool `json:"ask_job_type_on_launch"`
	AskLimitOnLaunch                bool `json:"ask_limit_on_launch"`
	AskInventoryOnLaunch            bool `json:"ask_inventory_on_launch"`
	AskCredentialOnLaunch           bool `json:"ask_credential_on_launch"`
	AskExecutionEnvironmentOnLaunch bool `json:"ask_execution_environment_on_launch"`
	AskLabelsOnLaunch               bool `json:"ask_labels_on_launch"`
	AskForksOnLaunch                bool `json:"ask_forks_on_launch"`
	AskDiffModeOnLaunch             bool `json:"ask_diff_mode_on_launch"`
	AskVerbosityOnLaunch            bool `json:"ask_verbosity_on_launch"`
	AskInstanceGroupsOnLaunch       bool `json:"ask_instance_groups_on_launch"`
	AskTimeoutOnLaunch              bool `json:"ask_timeout_on_launch"`
	AskJobSliceCountOnLaunch        bool `json:"ask_job_slice_count_on_launch"`
}

// JobLaunchRequestModel represents the request body for POST /job_templates/{id}/launch.
// This is separate from JobAPIModel because the POST request has different field formats.
// Note: credentials, labels, and instance_groups are all sent as arrays of integer IDs [N, M].
type JobLaunchRequestModel struct {
	Inventory            int64   `json:"inventory,omitempty"`
	ExtraVars            string  `json:"extra_vars,omitempty"`
	Limit                string  `json:"limit,omitempty"`
	JobTags              string  `json:"job_tags,omitempty"`
	SkipTags             string  `json:"skip_tags,omitempty"`
	DiffMode             bool    `json:"diff_mode,omitempty"`
	Verbosity            int64   `json:"verbosity,omitempty"`
	ExecutionEnvironment int64   `json:"execution_environment,omitempty"`
	Forks                int64   `json:"forks,omitempty"`
	JobSliceCount        int64   `json:"job_slice_count,omitempty"`
	Timeout              int64   `json:"timeout,omitempty"`
	InstanceGroups       []int64 `json:"instance_groups,omitempty"`
	Credentials          []int64 `json:"credentials,omitempty"`
	Labels               []int64 `json:"labels,omitempty"`
}

// JobModel are the attributes that are provided by the user and also used by the action.
type JobModel struct {
	TemplateID               types.Int64                      `tfsdk:"job_template_id"`
	InventoryID              types.Int64                      `tfsdk:"inventory_id"`
	Credentials              types.List                       `tfsdk:"credentials"`
	Labels                   types.List                       `tfsdk:"labels"`
	ExtraVars                customtypes.AAPCustomStringValue `tfsdk:"extra_vars"`
	WaitForCompletion        types.Bool                       `tfsdk:"wait_for_completion"`
	WaitForCompletionTimeout types.Int64                      `tfsdk:"wait_for_completion_timeout_seconds"`
	Limit                    customtypes.AAPCustomStringValue `tfsdk:"limit"`
	JobTags                  customtypes.AAPCustomStringValue `tfsdk:"job_tags"`
	SkipTags                 customtypes.AAPCustomStringValue `tfsdk:"skip_tags"`
	DiffMode                 types.Bool                       `tfsdk:"diff_mode"`
	Verbosity                types.Int64                      `tfsdk:"verbosity"`
	ExecutionEnvironmentID   types.Int64                      `tfsdk:"execution_environment"`
	Forks                    types.Int64                      `tfsdk:"forks"`
	JobSliceCount            types.Int64                      `tfsdk:"job_slice_count"`
	Timeout                  types.Int64                      `tfsdk:"timeout"`
	InstanceGroups           types.List                       `tfsdk:"instance_groups"`
}

// JobResourceModel maps the resource schema data.
type JobResourceModel struct {
	JobModel
	Status        types.String `tfsdk:"status"`
	Type          types.String `tfsdk:"job_type"`
	URL           types.String `tfsdk:"url"`
	IgnoredFields types.List   `tfsdk:"ignored_fields"`
	Triggers      types.Map    `tfsdk:"triggers"`
}

// JobResource is the resource implementation.
type JobResource struct {
	client ProviderHTTPClient
}

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource              = &JobResource{}
	_ resource.ResourceWithConfigure = &JobResource{}
)

var keyMapping = map[string]string{
	"inventory": "inventory",
}

// NewJobResource is a helper function to simplify the provider implementation.
func NewJobResource() resource.Resource {
	return &JobResource{}
}

// IsFinalStateAAPJob returns `true` given a string with the name of an AAP Job state
// if such state is final and cannot transition further; a.k.a, the job is completed.
func IsFinalStateAAPJob(state string) bool {
	finalStates := map[string]bool{
		"new":                 false,
		"pending":             false,
		"waiting":             false,
		"running":             false,
		statusSuccessfulConst: true,
		"failed":              true,
		"error":               true,
		"canceled":            true,
	}
	result, isPresent := finalStates[state]
	return isPresent && result
}

type RetryProgressFunc func(status string)

func retryUntilAAPJobReachesAnyFinalState(
	ctx context.Context,
	client ProviderHTTPClient,
	retryProgressFunc RetryProgressFunc,
	url string,
	status *string,
) retry.RetryFunc {
	return func() *retry.RetryError {
		responseBody, diagnostics := client.Get(url)
		if diagnostics.HasError() {
			return retry.RetryableError(fmt.Errorf("error fetching job status: %s", diagnostics.Errors()))
		}

		var statusResponse map[string]interface{}
		err := json.Unmarshal(responseBody, &statusResponse)
		if err != nil {
			return retry.RetryableError(fmt.Errorf("error fetching job status: %s", diagnostics.Errors()))
		}

		s, ok := statusResponse["status"].(string)
		if !ok {
			return retry.RetryableError(fmt.Errorf("error extracting job status: %s", "Could not extract status from response"))
		}
		*status = s
		tflog.Debug(ctx, "Job status update", statusResponse)

		retryProgressFunc(s)

		if !IsFinalStateAAPJob(s) {
			return retry.RetryableError(fmt.Errorf("job at: %s hasn't yet reached a final state. Current state: %s", url, s))
		}
		return nil
	}
}

// Metadata returns the resource type name.
func (r *JobResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_job"
}

// Configure adds the provider configured client to the data source.
func (r *JobResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// Schema defines the schema for the  jobresource.
func (r *JobResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	// Start with common attributes
	attributes := CommonJobSchemaAttributes("job")

	// Add job-specific attributes
	attributes["job_template_id"] = schema.Int64Attribute{
		Required:    true,
		Description: "ID of the job template.",
	}
	attributes["inventory_id"] = schema.Int64Attribute{
		Optional: true,
		Computed: true,
		PlanModifiers: []planmodifier.Int64{
			int64planmodifier.UseStateForUnknown(),
		},
		Description: "Identifier for the inventory where job should be created in. " +
			"If not provided, the job will be created in the default inventory.",
	}
	attributes["diff_mode"] = schema.BoolAttribute{
		Description: "Enable diff mode for the job run. When enabled, any module that supports diff mode will report the changes made.",
		Optional:    true,
		Computed:    true,
		PlanModifiers: []planmodifier.Bool{
			boolplanmodifier.UseStateForUnknown(),
		},
	}
	attributes["verbosity"] = schema.Int64Attribute{
		Description: "Verbosity level for the job run. Valid values: 0 (Normal), 1 (Verbose), 2 (More Verbose), 3 (Debug), 4 (Connection Debug), 5 (WinRM Debug).",
		Optional:    true,
		Computed:    true,
		Validators: []validator.Int64{
			int64validator.Between(0, VerbosityMax),
		},
		PlanModifiers: []planmodifier.Int64{
			int64planmodifier.UseStateForUnknown(),
		},
	}
	attributes["execution_environment"] = schema.Int64Attribute{
		Description: "ID of the execution environment to use for the job run.",
		Optional:    true,
		Computed:    true,
	}
	attributes["forks"] = schema.Int64Attribute{
		Description: "Number of parallel processes to use for the job run.",
		Optional:    true,
		Computed:    true,
		PlanModifiers: []planmodifier.Int64{
			int64planmodifier.UseStateForUnknown(),
		},
	}
	attributes["job_slice_count"] = schema.Int64Attribute{
		Description: "Number of slices to divide the job into.",
		Optional:    true,
		Computed:    true,
		PlanModifiers: []planmodifier.Int64{
			int64planmodifier.UseStateForUnknown(),
		},
	}
	attributes["timeout"] = schema.Int64Attribute{
		Description: "Timeout in seconds for the job run.",
		Optional:    true,
		Computed:    true,
		PlanModifiers: []planmodifier.Int64{
			int64planmodifier.UseStateForUnknown(),
		},
	}
	attributes["instance_groups"] = schema.ListAttribute{
		Description: "List of instance group IDs to use for the job run.",
		Optional:    true,
		Computed:    true,
		ElementType: types.Int64Type,
		PlanModifiers: []planmodifier.List{
			listplanmodifier.UseStateForUnknown(),
		},
	}
	attributes["credentials"] = schema.ListAttribute{
		Description: "List of credential IDs to use for the job run. (Write-only: value is sent to API but not returned in state)",
		Optional:    true,
		WriteOnly:   true,
		ElementType: types.Int64Type,
	}

	resp.Schema = schema.Schema{
		Attributes: attributes,
		MarkdownDescription: "Launches an AAP job.\n\n" +
			"A job is launched only when the resource is first created or when the " +
			"resource is changed. The " + "`triggers`" + " argument can be used to " +
			"launch a new job based on any arbitrary value.\n\n" +
			"This resource always creates a new job in AAP. A destroy will not " +
			"delete a job created by this resource, it will only remove the resource " +
			"from the state.\n\n" +
			"Moreover, you can set `wait_for_completion` to true, then Terraform will " +
			"wait until this job is created and reaches any final state before continuing. " +
			"This parameter works in both create and update operations.\n\n" +
			"You can also tweak `wait_for_completion_timeout_seconds` to control the timeout limit.",
	}
}

// Create creates a new job resource.
func (r *JobResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data JobResourceModel

	// Read Terraform plan data into job resource model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// WriteOnly attributes (credentials, labels) must be read from the config,
	// not the plan, because WriteOnly values are always null in the plan.
	var configData JobResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &configData)...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Credentials = configData.Credentials
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

func (r *JobResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data JobResourceModel
	var diags diag.Diagnostics

	// Read current Terraform state data into job resource model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get latest job data from AAP
	readResponseBody, diags, status := r.client.GetWithStatus(data.URL.ValueString(), nil)

	// Check if the response is 404, meaning the job does not exist and should be recreated
	if status == http.StatusNotFound {
		resp.Diagnostics.AddWarning(
			"Job not found",
			"The job was not found. It may have been deleted. The job will be recreated.",
		)
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save latest hob data into job resource model
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

// Update updates an existing job resource.
func (r *JobResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data JobResourceModel

	// Read Terraform plan data into job resource model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// WriteOnly attributes (credentials, labels) must be read from the config,
	// not the plan, because WriteOnly values are always null in the plan.
	var configData JobResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &configData)...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Credentials = configData.Credentials
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
func (r JobResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

// CreateRequestBody creates a JSON encoded request body from the job resource data.
// Null/unknown fields return zero values which are omitted via omitempty JSON tags.
func (r *JobModel) CreateRequestBody() ([]byte, diag.Diagnostics) {
	var diags diag.Diagnostics

	job := JobLaunchRequestModel{
		ExtraVars:            r.ExtraVars.ValueString(),
		Limit:                r.Limit.ValueString(),
		JobTags:              r.JobTags.ValueString(),
		SkipTags:             r.SkipTags.ValueString(),
		Inventory:            r.InventoryID.ValueInt64(),
		Verbosity:            r.Verbosity.ValueInt64(),
		ExecutionEnvironment: r.ExecutionEnvironmentID.ValueInt64(),
		Forks:                r.Forks.ValueInt64(),
		JobSliceCount:        r.JobSliceCount.ValueInt64(),
		Timeout:              r.Timeout.ValueInt64(),
		DiffMode:             r.DiffMode.ValueBool(),
		InstanceGroups:       ConvertListToInt64Slice(r.InstanceGroups),
		Credentials:          ConvertListToInt64Slice(r.Credentials),
		Labels:               ConvertListToInt64Slice(r.Labels),
	}

	// Create JSON encoded request body
	jsonBody, err := json.Marshal(job)
	if err != nil {
		diags.AddError(
			"Error marshaling request body",
			fmt.Sprintf("Could not create request body for job resource, unexpected error: %s", err.Error()),
		)
		return nil, diags
	}
	return jsonBody, diags
}

// ParseHTTPResponse updates the job resource data from an AAP API response.
func (r *JobResourceModel) ParseHTTPResponse(body []byte) diag.Diagnostics {
	var diags diag.Diagnostics

	// Unmarshal the JSON response
	var resultAPIJob JobAPIModel
	err := json.Unmarshal(body, &resultAPIJob)
	if err != nil {
		diags.AddError("Error parsing JSON response from AAP", err.Error())
		return diags
	}

	// Map response to the job resource schema and update attribute values
	// All Optional+Computed fields use UseStateForUnknown() plan modifiers,
	// so we can safely set values from the API response without causing drift
	r.Type = types.StringValue(resultAPIJob.Type)
	r.URL = types.StringValue(resultAPIJob.URL)
	r.Status = types.StringValue(resultAPIJob.Status)
	r.TemplateID = types.Int64Value(resultAPIJob.TemplateID)
	r.InventoryID = types.Int64Value(resultAPIJob.Inventory)
	r.Limit = customtypes.NewAAPCustomStringValue(resultAPIJob.Limit)
	r.JobTags = customtypes.NewAAPCustomStringValue(resultAPIJob.JobTags)
	r.SkipTags = customtypes.NewAAPCustomStringValue(resultAPIJob.SkipTags)
	r.DiffMode = types.BoolValue(resultAPIJob.DiffMode)
	r.Verbosity = types.Int64Value(resultAPIJob.Verbosity)
	r.ExecutionEnvironmentID = types.Int64Value(resultAPIJob.ExecutionEnvironment)
	r.Forks = types.Int64Value(resultAPIJob.Forks)
	r.JobSliceCount = types.Int64Value(resultAPIJob.JobSliceCount)
	r.Timeout = types.Int64Value(resultAPIJob.Timeout)

	// InstanceGroups requires special handling: the API returns a single instance_group,
	// but user may have configured multiple. Only set from API if user didn't provide a value.
	if r.InstanceGroups.IsNull() || r.InstanceGroups.IsUnknown() {
		if resultAPIJob.InstanceGroup != 0 {
			r.InstanceGroups, _ = types.ListValue(types.Int64Type, []attr.Value{types.Int64Value(resultAPIJob.InstanceGroup)})
		} else {
			r.InstanceGroups = types.ListNull(types.Int64Type)
		}
	}

	// Credentials and Labels are WriteOnly and handled separately via API
	diags = r.ParseIgnoredFields(resultAPIJob.IgnoredFields)
	return diags
}

// ParseIgnoredFields parses ignored fields from the AAP API response.
func (r *JobResourceModel) ParseIgnoredFields(ignoredFields map[string]interface{}) (diags diag.Diagnostics) {
	r.IgnoredFields, diags = ParseIgnoredFieldsToList(ignoredFields)
	return diags
}

// LaunchJob launches a job from the Job Template. It first checks if the job can be launched,
// then POSTs to launch the job.
func (r *JobModel) LaunchJob(client ProviderHTTPClient) (body []byte, diags diag.Diagnostics) {
	// First, check if the job can be launched
	diags = r.CanJobBeLaunched(client)
	if diags.HasError() {
		return nil, diags
	}

	// Use shared launch helper
	return LaunchJobTemplate(client, "job_templates", r)
}

// GetLaunchJob performs a GET request to the Job Template launch endpoint to retrieve
// the launch configuration.
func (r *JobModel) GetLaunchJob(client ProviderHTTPClient) (launchConfig JobLaunchAPIModel, diags diag.Diagnostics) {
	diags = GetLaunchConfiguration(client, "job_templates", r.TemplateID.ValueInt64(), &launchConfig, "Job Template")
	return launchConfig, diags
}

// CanJobBeLaunched retrieves the launch configuration and validates that all required
// fields are provided. It also warns when fields are provided but will be ignored.
// This determines if a Job Template can be launched.
func (r *JobModel) CanJobBeLaunched(client ProviderHTTPClient) (diags diag.Diagnostics) {
	launchConfig, diags := r.GetLaunchJob(client)
	if diags.HasError() {
		return diags
	}

	validations := []LaunchFieldValidation{
		{launchConfig.AskVariablesOnLaunch, r.ExtraVars, "extra_vars"},
		{launchConfig.AskTagsOnLaunch, r.JobTags, "job_tags"},
		{launchConfig.AskSkipTagsOnLaunch, r.SkipTags, "skip_tags"},
		{launchConfig.AskDiffModeOnLaunch, r.DiffMode, "diff_mode"},
		{launchConfig.AskLimitOnLaunch, r.Limit, "limit"},
		{launchConfig.AskInventoryOnLaunch, r.InventoryID, "inventory_id"},
		{launchConfig.AskCredentialOnLaunch, r.Credentials, "credentials"},
		{launchConfig.AskExecutionEnvironmentOnLaunch, r.ExecutionEnvironmentID, "execution_environment"},
		{launchConfig.AskLabelsOnLaunch, r.Labels, "labels"},
		{launchConfig.AskForksOnLaunch, r.Forks, "forks"},
		{launchConfig.AskVerbosityOnLaunch, r.Verbosity, "verbosity"},
		{launchConfig.AskInstanceGroupsOnLaunch, r.InstanceGroups, "instance_groups"},
		{launchConfig.AskTimeoutOnLaunch, r.Timeout, "timeout"},
		{launchConfig.AskJobSliceCountOnLaunch, r.JobSliceCount, "job_slice_count"},
	}

	diags.Append(ValidateLaunchFields(validations, "Job Template")...)

	return diags
}

// LaunchJobWithResponse launches a job from the job template and parses the HTTP response
// into the JobResourceModel fields.
func (r *JobResourceModel) LaunchJobWithResponse(client ProviderHTTPClient) diag.Diagnostics {
	body, diags := r.LaunchJob(client)
	if diags.HasError() {
		return diags
	}
	return r.ParseHTTPResponse(body)
}

// launchAndWait launches a job and optionally waits for completion.
// This is shared logic between Create and Update operations.
func (r *JobResource) launchAndWait(
	ctx context.Context,
	data *JobResourceModel,
) diag.Diagnostics {
	var diags diag.Diagnostics

	// Launch the job
	diags.Append(data.LaunchJobWithResponse(r.client)...)
	if diags.HasError() {
		return diags
	}

	// Wait for completion if configured
	if data.WaitForCompletion.ValueBool() {
		timeout := time.Duration(data.WaitForCompletionTimeout.ValueInt64()) * time.Second
		var status string
		retryProgressFunc := func(status string) {
			tflog.Debug(ctx, "Job status update", map[string]interface{}{
				"status": status,
				"url":    data.URL.ValueString(),
			})
		}
		err := retry.RetryContext(ctx, timeout, retryUntilAAPJobReachesAnyFinalState(ctx, r.client, retryProgressFunc, data.URL.ValueString(), &status))
		if err != nil {
			diags.Append(diag.NewErrorDiagnostic("error when waiting for AAP job to complete", err.Error()))
		}
		if diags.HasError() {
			return diags
		}
		data.Status = types.StringValue(status)
	}

	return diags
}

// GetTemplateID implements the LaunchableJob interface.
func (r *JobModel) GetTemplateID() int64 {
	return r.TemplateID.ValueInt64()
}
