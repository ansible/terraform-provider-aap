package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"path"

	"github.com/ansible/terraform-provider-aap/internal/provider/customtypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const (
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
	ScmBranch            string                 `json:"scm_branch,omitempty"`
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

// CreateRequestBody creates a JSON encoded request body from the job resource data.
// Null/unknown fields return zero values which are omitted via omitempty JSON tags.
func (r *JobModel) CreateRequestBody() ([]byte, diag.Diagnostics) {
	var diags diag.Diagnostics

	req := JobLaunchRequestModel{
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

	jsonBody, err := json.Marshal(req)
	if err != nil {
		diags.AddError(
			"Error marshaling request body",
			fmt.Sprintf("Could not create request body for job resource, unexpected error: %s", err.Error()),
		)
		return nil, diags
	}
	return jsonBody, diags
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

// LaunchJob launches a job from the Job Template. It first checks if the job can be launched,
// then POSTs to launch the job.
func (r *JobModel) LaunchJob(client ProviderHTTPClient) (postBody []byte, diags diag.Diagnostics) {
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

	// POST to launch the job
	var launchURL = path.Join(client.getAPIEndpoint(), "job_templates", r.TemplateID.String(), "launch")
	requestData := bytes.NewReader(requestBody)
	postResp, postBody, postErr := client.doRequest(http.MethodPost, launchURL, nil, requestData)
	diags.Append(ValidateResponse(postResp, postBody, postErr, []int{http.StatusCreated})...)
	if diags.HasError() {
		return nil, diags
	}

	return postBody, diags
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
