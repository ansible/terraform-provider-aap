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
	AskVariablesOnLaunch   bool     `json:"ask_variables_on_launch"`
	AskTagsOnLaunch        bool     `json:"ask_tags_on_launch"`
	AskSkipTagsOnLaunch    bool     `json:"ask_skip_tags_on_launch"`
	AskLimitOnLaunch       bool     `json:"ask_limit_on_launch"`
	AskInventoryOnLaunch   bool     `json:"ask_inventory_on_launch"`
	AskLabelsOnLaunch      bool     `json:"ask_labels_on_launch"`
	SurveyEnabled          bool     `json:"survey_enabled"`
	VariablesNeededToStart []string `json:"variables_needed_to_start"`
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

// GetLaunchWorkflowJob performs a GET request to the Workflow Job Template launch endpoint to retrieve
// the launch configuration.
func (r *WorkflowJobModel) GetLaunchWorkflowJob(client ProviderHTTPClient) (launchConfig WorkflowJobLaunchAPIModel, diags diag.Diagnostics) {
	var launchURL = path.Join(client.getAPIEndpoint(), "workflow_job_templates", r.TemplateID.String(), "launch")

	getResp, getBody, getErr := client.doRequest(http.MethodGet, launchURL, nil, nil)
	diags.Append(ValidateResponse(getResp, getBody, getErr, []int{http.StatusOK})...)
	if diags.HasError() {
		return launchConfig, diags
	}

	err := json.Unmarshal(getBody, &launchConfig)
	if err != nil {
		diags.AddError(
			"Error parsing Workflow Job Template launch configuration",
			fmt.Sprintf("Could not parse launch configuration response: %s", err.Error()),
		)
		return launchConfig, diags
	}

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

	validations := []struct {
		askOnLaunch bool
		isNull      bool
		fieldName   string
	}{
		{launchConfig.AskVariablesOnLaunch, r.ExtraVars.IsNull(), "extra_vars"},
		{launchConfig.AskTagsOnLaunch, r.JobTags.IsNull(), "job_tags"},
		{launchConfig.AskSkipTagsOnLaunch, r.SkipTags.IsNull(), "skip_tags"},
		{launchConfig.AskLimitOnLaunch, r.Limit.IsNull(), "limit"},
		{launchConfig.AskInventoryOnLaunch, r.InventoryID.IsNull(), "inventory_id"},
		{launchConfig.AskLabelsOnLaunch, r.Labels.IsNull(), "labels"},
	}

	for _, v := range validations {
		if v.askOnLaunch && v.isNull {
			diags.AddError(
				"Missing required field",
				fmt.Sprintf("Workflow Job Template requires '%s' to be provided at launch", v.fieldName),
			)
		}
		if !v.askOnLaunch && !v.isNull {
			diags.AddWarning(
				"Field will be ignored",
				fmt.Sprintf("'%s' is provided but the Workflow Job Template does not allow it to be specified at launch", v.fieldName),
			)
		}
	}

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

	// Create request body from workflow job data
	requestBody, diagCreateReq := r.CreateRequestBody()
	diags.Append(diagCreateReq...)
	if diags.HasError() {
		return nil, diags
	}

	requestData := bytes.NewReader(requestBody)
	var postURL = path.Join(client.getAPIEndpoint(), "workflow_job_templates", r.TemplateID.String(), "launch")
	resp, body, err := client.doRequest(http.MethodPost, postURL, nil, requestData)
	diags.Append(ValidateResponse(resp, body, err, []int{http.StatusCreated})...)
	if diags.HasError() {
		return nil, diags
	}

	return body, diags
}
