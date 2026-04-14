package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"time"

	"github.com/ansible/terraform-provider-aap/internal/provider/customtypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
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
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"job_template_id": schema.Int64Attribute{
				Required:    true,
				Description: "ID of the job template.",
			},
			"inventory_id": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
				Description: "Identifier for the inventory where job should be created in. " +
					"If not provided, the job will be created in the default inventory.",
			},
			"job_type": schema.StringAttribute{
				Computed:    true,
				Description: "Job type",
			},
			"url": schema.StringAttribute{
				Computed:    true,
				Description: "URL of the job template",
			},
			"status": schema.StringAttribute{
				Computed:    true,
				Description: "Status of the job",
			},
			"extra_vars": schema.StringAttribute{
				Description: "Extra Variables. Must be provided as either a JSON or YAML string.",
				Optional:    true,
				CustomType:  customtypes.AAPCustomStringType{},
			},
			"triggers": schema.MapAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Map of arbitrary keys and values that, when changed, will trigger a creation" +
					" of a new Job on AAP. Use 'terraform taint' if you want to force the creation of a new job" +
					" without changing this value.",
			},
			"ignored_fields": schema.ListAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "The list of properties set by the user but ignored on server side.",
			},
			"limit": schema.StringAttribute{
				Description: "Limit pattern to restrict the job run to specific hosts.",
				Optional:    true,
				Computed:    true,
				CustomType:  customtypes.AAPCustomStringType{},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"job_tags": schema.StringAttribute{
				Description: "Tags to include in the job run.",
				Optional:    true,
				Computed:    true,
				CustomType:  customtypes.AAPCustomStringType{},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"skip_tags": schema.StringAttribute{
				Description: "Tags to skip in the job run.",
				Optional:    true,
				Computed:    true,
				CustomType:  customtypes.AAPCustomStringType{},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"diff_mode": schema.BoolAttribute{
				Description: "Enable diff mode for the job run. When enabled, any module that supports diff mode will report the changes made.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"verbosity": schema.Int64Attribute{
				Description: "Verbosity level for the job run. Valid values: 0 (Normal), 1 (Verbose), 2 (More Verbose), 3 (Debug), 4 (Connection Debug), 5 (WinRM Debug).",
				Optional:    true,
				Computed:    true,
				Validators: []validator.Int64{
					int64validator.Between(0, VerbosityMax),
				},
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"execution_environment": schema.Int64Attribute{
				Description: "ID of the execution environment to use for the job run.",
				Optional:    true,
				Computed:    true,
			},
			"forks": schema.Int64Attribute{
				Description: "Number of parallel processes to use for the job run.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"job_slice_count": schema.Int64Attribute{
				Description: "Number of slices to divide the job into.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"timeout": schema.Int64Attribute{
				Description: "Timeout in seconds for the job run.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			// instance_groups is marked as Computed because the API may return a different
			// value than what the user configured. When launching a job, the user can specify
			// multiple instance groups, but the API response only includes the single instance
			// group that was actually assigned. UseStateForUnknown preserves the user's
			// configured value in state to prevent perpetual drift.
			"instance_groups": schema.ListAttribute{
				Description: "List of instance group IDs to use for the job run.",
				Optional:    true,
				Computed:    true,
				ElementType: types.Int64Type,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			// credentials is marked as WriteOnly because the value is sent to the API
			// at job launch time but is not returned from the job GET endpoint.
			// The actual credentials are managed via the `related` API endpoint.
			"credentials": schema.ListAttribute{
				Description: "List of credential IDs to use for the job run. (Write-only: value is sent to API but not returned in state)",
				Optional:    true,
				WriteOnly:   true,
				ElementType: types.Int64Type,
			},
			// labels is marked as WriteOnly because the value is sent to the API
			// at job launch time but is not returned from the job GET endpoint.
			// The actual labels are managed via the `related` API endpoint.
			"labels": schema.ListAttribute{
				Description: "List of label IDs to apply to the job. (Write-only: value is sent to API but not returned in state)",
				Optional:    true,
				WriteOnly:   true,
				ElementType: types.Int64Type,
			},
			"wait_for_completion": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
				Description: "When this is set to `true`, Terraform will wait until this aap_job resource is created, reaches " +
					"any final status and then, proceeds with the following resource operation",
			},
			"wait_for_completion_timeout_seconds": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				Default:  int64default.StaticInt64(waitForCompletionTimeoutDefault),
				Description: "Sets the maximum amount of seconds Terraform will wait before timing out the updates, " +
					"and the job creation will fail. Default value of `120`",
			},
		},
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

	resp.Diagnostics.Append(data.LaunchJobWithResponse(r.client)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If the job was configured to wait for completion, start polling the job status
	// and wait for it to complete before marking the resource as created
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
			resp.Diagnostics.Append(diag.NewErrorDiagnostic("error when waiting for AAP job to complete", err.Error()))
		}
		if resp.Diagnostics.HasError() {
			return
		}
		data.Status = types.StringValue(status)
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

	// Create new Job from job template
	resp.Diagnostics.Append(data.LaunchJobWithResponse(r.client)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If the job was configured to wait for completion, start polling the job status
	// and wait for it to complete before marking the resource as created
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
			resp.Diagnostics.Append(diag.NewErrorDiagnostic("error when waiting for AAP job to complete", err.Error()))
		}
		if resp.Diagnostics.HasError() {
			return
		}
		data.Status = types.StringValue(status)
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
	r.IgnoredFields = types.ListNull(types.StringType)
	var keysList = []attr.Value{}

	for k := range ignoredFields {
		key := k
		if v, ok := keyMapping[k]; ok {
			key = v
		}
		keysList = append(keysList, types.StringValue(key))
	}

	if len(keysList) > 0 {
		r.IgnoredFields, _ = types.ListValue(types.StringType, keysList)
	}

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

	// Create request body from job data
	requestBody, diagCreateReq := r.CreateRequestBody()
	diags.Append(diagCreateReq...)
	if diags.HasError() {
		return nil, diags
	}

	requestData := bytes.NewReader(requestBody)
	var postURL = path.Join(client.getAPIEndpoint(), "job_templates", r.TemplateID.String(), "launch")
	resp, body, err := client.doRequest(http.MethodPost, postURL, nil, requestData)
	diags.Append(ValidateResponse(resp, body, err, []int{http.StatusCreated})...)
	if diags.HasError() {
		return nil, diags
	}

	return body, diags
}

// GetLaunchJob performs a GET request to the Job Template launch endpoint to retrieve
// the launch configuration.
func (r *JobModel) GetLaunchJob(client ProviderHTTPClient) (launchConfig JobLaunchAPIModel, diags diag.Diagnostics) {
	var launchURL = path.Join(client.getAPIEndpoint(), "job_templates", r.TemplateID.String(), "launch")

	getResp, getBody, getErr := client.doRequest(http.MethodGet, launchURL, nil, nil)
	diags.Append(ValidateResponse(getResp, getBody, getErr, []int{http.StatusOK})...)
	if diags.HasError() {
		return launchConfig, diags
	}

	err := json.Unmarshal(getBody, &launchConfig)
	if err != nil {
		diags.AddError(
			"Error parsing Job Template launch configuration",
			fmt.Sprintf("Could not parse launch configuration response: %s", err.Error()),
		)
		return launchConfig, diags
	}

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

	validations := []struct {
		askOnLaunch bool
		isNull      bool
		fieldName   string
	}{
		{launchConfig.AskVariablesOnLaunch, r.ExtraVars.IsNull(), "extra_vars"},
		{launchConfig.AskTagsOnLaunch, r.JobTags.IsNull(), "job_tags"},
		{launchConfig.AskSkipTagsOnLaunch, r.SkipTags.IsNull(), "skip_tags"},
		{launchConfig.AskDiffModeOnLaunch, r.DiffMode.IsNull(), "diff_mode"},
		{launchConfig.AskLimitOnLaunch, r.Limit.IsNull(), "limit"},
		{launchConfig.AskInventoryOnLaunch, r.InventoryID.IsNull(), "inventory_id"},
		{launchConfig.AskCredentialOnLaunch, r.Credentials.IsNull(), "credentials"},
		{launchConfig.AskExecutionEnvironmentOnLaunch, r.ExecutionEnvironmentID.IsNull(), "execution_environment"},
		{launchConfig.AskLabelsOnLaunch, r.Labels.IsNull(), "labels"},
		{launchConfig.AskForksOnLaunch, r.Forks.IsNull(), "forks"},
		{launchConfig.AskVerbosityOnLaunch, r.Verbosity.IsNull(), "verbosity"},
		{launchConfig.AskInstanceGroupsOnLaunch, r.InstanceGroups.IsNull(), "instance_groups"},
		{launchConfig.AskTimeoutOnLaunch, r.Timeout.IsNull(), "timeout"},
		{launchConfig.AskJobSliceCountOnLaunch, r.JobSliceCount.IsNull(), "job_slice_count"},
	}

	for _, v := range validations {
		if v.askOnLaunch && v.isNull {
			diags.AddError(
				"Missing required field",
				fmt.Sprintf("Job Template requires '%s' to be provided at launch", v.fieldName),
			)
		}
		if !v.askOnLaunch && !v.isNull {
			diags.AddWarning(
				"Field will be ignored",
				fmt.Sprintf("'%s' is provided but the Job Template does not allow it to be specified at launch", v.fieldName),
			)
		}
	}

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
