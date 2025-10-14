package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ansible/terraform-provider-aap/internal/provider/customtypes"
	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
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
	var config JobModel

	response.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if response.Diagnostics.HasError() {
		return
	}

	// Set default timeout if not provided
	if config.WaitForCompletionTimeout.IsNull() {
		config.WaitForCompletionTimeout = types.Int64Value(waitForCompletionTimeoutDefault)
	}

	body, diags := config.LaunchJob(a.client)
	if diags.HasError() {
		response.Diagnostics.Append(diags...)
		return
	}

	// Parse response to get job details
	var jobResponse JobAPIModel
	err := json.Unmarshal(body, &jobResponse)
	if err != nil {
		response.Diagnostics.AddError("Error parsing JSON response from AAP", err.Error())
		return
	}

	// Extract job URL for polling if wait_for_completion is enabled
	if config.WaitForCompletion.ValueBool() {
		timeout := time.Duration(config.WaitForCompletionTimeout.ValueInt64()) * time.Second
		var status string
		err := retry.RetryContext(ctx, timeout, retryUntilAAPJobReachesAnyFinalState(ctx, a.client, jobResponse.URL, &status))
		if err != nil {
			response.Diagnostics.Append(diag.NewErrorDiagnostic("error when waiting for AAP job to complete", err.Error()))
		}
		if response.Diagnostics.HasError() {
			return
		}
		jobResponse.Status = status
	}
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
