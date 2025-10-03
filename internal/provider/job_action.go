package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"strconv"
	"time"

	"github.com/ansible/terraform-provider-aap/internal/provider/customtypes"
	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// JobAction represents a job action that can be executed in AAP.
type JobAction struct {
	client ProviderHTTPClient
}

func NewJobAction() action.Action {
	return &JobAction{}
}

var (
	_ action.Action = (*JobAction)(nil)
)

// Schema defines the schema for the job action
func (a *JobAction) Schema(_ context.Context, _ action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"job_template_id": schema.Int64Attribute{
				Required:    true,
				Description: "ID of the job template.",
			},
			"inventory_id": schema.Int64Attribute{
				Optional: true,
				Description: "Identifier for the inventory where job should be created in. " +
					"If not provided, the job will be created in the default inventory.",
			},
			"extra_vars": schema.StringAttribute{
				Description: "Extra Variables. Must be provided as either a JSON or YAML string.",
				Optional:    true,
				CustomType:  customtypes.AAPCustomStringType{},
			},
			"wait_for_completion": schema.BoolAttribute{
				Optional: true,
				Description: "When this is set to `true`, Terraform will wait until this aap_job resource is created, reaches " +
					"any final status and then, proceeds with the following resource operation",
			},
			"wait_for_completion_timeout_seconds": schema.Int64Attribute{
				Optional: true,
				Description: "Sets the maximum amount of seconds Terraform will wait before timing out the updates, " +
					"and the job creation will fail. Default value of `120`",
			},
		},
		MarkdownDescription: "Launches an AAP job.\n\n" +
			"This actions always creates a new job in AAP. \n" +
			"Moreover, you can set `wait_for_completion` to true, then Terraform will " +
			"wait until this job is created and reaches any final state before continuing. " +
			"You can also tweak `wait_for_completion_timeout_seconds` to control the timeout limit.",
	}
}

// Invoke executes the job action.
func (a *JobAction) Invoke(ctx context.Context, req action.InvokeRequest, response *action.InvokeResponse) {
	var config struct {
		JobTemplateID            types.Int64                      `tfsdk:"job_template_id"`
		InventoryID              types.Int64                      `tfsdk:"inventory_id"`
		ExtraVars                customtypes.AAPCustomStringValue `tfsdk:"extra_vars"`
		WaitForCompletion        types.Bool                       `tfsdk:"wait_for_completion"`
		WaitForCompletionTimeout types.Int64                      `tfsdk:"wait_for_completion_timeout_seconds"`
	}

	response.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if response.Diagnostics.HasError() {
		return
	}

	// Set default timeout if not provided
	if config.WaitForCompletionTimeout.IsNull() {
		config.WaitForCompletionTimeout = types.Int64Value(waitForCompletionTimeoutDefault)
	}

	// Set up timeout context if wait for completion is enabled
	if config.WaitForCompletion.ValueBool() {
		c, cancel := context.WithTimeout(ctx, time.Duration(config.WaitForCompletionTimeout.ValueInt64())*time.Second)
		defer cancel()
		ctx = c
	}

	// Determine which template type to use
	postURL := path.Join(a.client.getApiEndpoint(), "job_templates", strconv.FormatInt(config.JobTemplateID.ValueInt64(), 10), "launch")

	// Create request body
	requestBody := map[string]interface{}{}
	if !config.ExtraVars.IsNull() {
		requestBody["extra_vars"] = config.ExtraVars.ValueString()
	}
	if !config.InventoryID.IsNull() && config.InventoryID.ValueInt64() != 0 {
		requestBody["inventory"] = config.InventoryID.ValueInt64()
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		response.Diagnostics.AddError(
			"Error marshaling request body",
			fmt.Sprintf("Could not create request body for job action, unexpected error: %s", err.Error()),
		)
		return
	}

	requestData := bytes.NewReader(jsonBody)
	resp, body, err := a.client.doRequest(http.MethodPost, postURL, nil, requestData)
	response.Diagnostics.Append(ValidateResponse(resp, body, err, []int{http.StatusCreated})...)
	if response.Diagnostics.HasError() {
		return
	}

	// Parse response to get job details
	var jobResponse map[string]interface{}
	err = json.Unmarshal(body, &jobResponse)
	if err != nil {
		response.Diagnostics.AddError("Error parsing JSON response from AAP", err.Error())
		return
	}

	// Extract job URL for polling if wait_for_completion is enabled
	if config.WaitForCompletion.ValueBool() {
		jobURL, ok := jobResponse["url"].(string)
		if !ok {
			response.Diagnostics.AddError("Error extracting job URL", "Could not extract job URL from response")
			return
		}
		response.Diagnostics.Append(a.waitForCompletion(ctx, jobURL)...)
	}
}

func (a *JobAction) waitForCompletion(ctx context.Context, jobURL string) diag.Diagnostics {
	for ctx.Err() == nil {
		responseBody, diagnostics := a.client.Get(jobURL)
		if diagnostics.HasError() {
			return diagnostics
		}

		var statusResponse map[string]interface{}
		err := json.Unmarshal(responseBody, &statusResponse)
		if err != nil {
			return diag.Diagnostics{diag.NewErrorDiagnostic("Error parsing status response", err.Error())}
		}

		status, ok := statusResponse["status"].(string)
		if !ok {
			return diag.Diagnostics{diag.NewErrorDiagnostic("Error extracting job status", "Could not extract status from response")}
		}

		if IsFinalStateAAPJob(status) {
			break
		}
		time.Sleep(1 * time.Second)
	}
	if ctx.Err() != nil {
		return diag.Diagnostics{diag.NewErrorDiagnostic("error when waiting for AAP job to complete", ctx.Err().Error())}
	}
	return nil
}

// Configure configures the job action with the provider client
func (a *JobAction) Configure(_ context.Context, req action.ConfigureRequest, resp *action.ConfigureResponse) {
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

	a.client = client
}

// Metadata returns the action metadata
func (a *JobAction) Metadata(_ context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_job"
}
