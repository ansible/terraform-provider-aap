package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ansible/terraform-provider-aap/internal/provider/customtypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
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

type JobActionModel struct {
	JobModel
	IgnoreJobResults types.Bool `tfsdk:"ignore_job_results"`
}

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
			"limit": schema.StringAttribute{
				Description: "Limit pattern to restrict the job run to specific hosts.",
				Optional:    true,
				CustomType:  customtypes.AAPCustomStringType{},
			},
			"job_tags": schema.StringAttribute{
				Description: "Tags to include in the job run.",
				Optional:    true,
				CustomType:  customtypes.AAPCustomStringType{},
			},
			"skip_tags": schema.StringAttribute{
				Description: "Tags to skip in the job run.",
				Optional:    true,
				CustomType:  customtypes.AAPCustomStringType{},
			},
			"diff_mode": schema.BoolAttribute{
				Description: "Enable diff mode for the job run. When enabled, any module that supports diff mode will report the changes made.",
				Optional:    true,
			},
			"verbosity": schema.Int64Attribute{
				Description: "Verbosity level for the job run. Valid values: 0 (Normal), 1 (Verbose), 2 (More Verbose), 3 (Debug), 4 (Connection Debug), 5 (WinRM Debug).",
				Optional:    true,
				Validators: []validator.Int64{
					int64validator.Between(0, VerbosityMax),
				},
			},
			"execution_environment": schema.Int64Attribute{
				Description: "ID of the execution environment to use for the job run.",
				Optional:    true,
			},
			"forks": schema.Int64Attribute{
				Description: "Number of parallel processes to use for the job run.",
				Optional:    true,
			},
			"job_slice_count": schema.Int64Attribute{
				Description: "Number of slices to divide the job into.",
				Optional:    true,
			},
			"timeout": schema.Int64Attribute{
				Description: "Timeout in seconds for the job run.",
				Optional:    true,
			},
			"instance_groups": schema.ListAttribute{
				Description: "List of instance group IDs to use for the job run.",
				Optional:    true,
				ElementType: types.Int64Type,
			},
			"credentials": schema.ListAttribute{
				Description: "List of credential IDs to use for the job run. (Value is sent to API but not returned in state)",
				Optional:    true,
				ElementType: types.Int64Type,
			},
			"labels": schema.ListAttribute{
				Description: "List of label IDs to apply to the job. (Value is sent to API but not returned in state)",
				Optional:    true,
				ElementType: types.Int64Type,
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
			"ignore_job_results": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "When this is set to `true`, and wait_for_completion is `true`, ignore the job status.",
			},
		},
		MarkdownDescription: "Launches an AAP job.\n\n" +
			"This action always creates a new job in AAP. \n" +
			"Moreover, you can set `wait_for_completion` to true, then Terraform will " +
			"wait until this job is created and reaches any final state before continuing. " +
			"You can also tweak `wait_for_completion_timeout_seconds` to control the timeout limit.",
	}
}

// Invoke executes the job action.
func (a *JobAction) Invoke(ctx context.Context, req action.InvokeRequest, response *action.InvokeResponse) {
	var config JobActionModel

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

	response.SendProgress(action.InvokeProgressEvent{
		Message: fmt.Sprintf("Job launched, URL: %s, Template ID: %d, Inventory ID: %d", jobResponse.URL, jobResponse.TemplateID, jobResponse.Inventory),
	})

	tflog.Debug(ctx, "job launched", map[string]interface{}{
		"url":            jobResponse.URL,
		"status":         jobResponse.Status,
		"type":           jobResponse.Type,
		"template_id":    jobResponse.TemplateID,
		"inventory_id":   jobResponse.Inventory,
		"extra_vars":     jobResponse.ExtraVars,
		"ignored_fields": jobResponse.IgnoredFields,
	})

	// Extract job URL for polling if wait_for_completion is enabled
	if config.WaitForCompletion.ValueBool() {
		if config.WaitForCompletionTimeout.IsNull() {
			config.WaitForCompletionTimeout = types.Int64Value(waitForCompletionTimeoutDefault)
		}
		timeout := time.Duration(config.WaitForCompletionTimeout.ValueInt64()) * time.Second
		var status string

		retryProgressFunc := func(status string) {
			response.SendProgress(action.InvokeProgressEvent{
				Message: fmt.Sprintf("Job at: %s is in status: %s", jobResponse.URL, status),
			})
		}
		err := retry.RetryContext(
			ctx,
			timeout,
			retryUntilAAPJobReachesAnyFinalState(ctx, a.client, retryProgressFunc, jobResponse.URL, &status),
		)
		if err != nil {
			response.Diagnostics.Append(diag.NewErrorDiagnostic("error when waiting for AAP job to complete", err.Error()))
			return
		}
		jobResponse.Status = status
		if status != statusSuccessfulConst {
			if config.IgnoreJobResults.ValueBool() {
				response.Diagnostics.Append(
					diag.NewWarningDiagnostic(
						fmt.Sprintf("AAP job %s", status),
						fmt.Sprintf("API Path: %s", jobResponse.URL),
					),
				)
			} else {
				response.Diagnostics.Append(
					diag.NewErrorDiagnostic(
						fmt.Sprintf("AAP job %s", status),
						fmt.Sprintf("API Path: %s", jobResponse.URL),
					),
				)
			}
		}
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
	resp.TypeName = req.ProviderTypeName + "_job_launch"
}
