package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ansible/terraform-provider-aap/internal/provider/customtypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
)

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
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"inventory_id": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
				Description: "Identifier for the inventory the job will be run against.",
			},
			"workflow_job_template_id": schema.Int64Attribute{
				Required:    true,
				Description: "ID of the workflow job template.",
			},
			"job_type": schema.StringAttribute{
				Computed:    true,
				Description: "Job type",
			},
			"url": schema.StringAttribute{
				Computed:    true,
				Description: "URL of the workflow job template",
			},
			"status": schema.StringAttribute{
				Computed:    true,
				Description: "Status of the workflow job",
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
					" of a new Workflow Job on AAP. Use 'terraform taint' if you want to force the creation of" +
					" a new workflow job without changing this value.",
			},
			"ignored_fields": schema.ListAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "The list of properties set by the user but ignored on server side.",
			},
			"limit": schema.StringAttribute{
				Description: "Limit pattern to restrict the workflow job run to specific hosts.",
				Optional:    true,
				Computed:    true,
				CustomType:  customtypes.AAPCustomStringType{},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"job_tags": schema.StringAttribute{
				Description: "Tags to include in the workflow job run.",
				Optional:    true,
				Computed:    true,
				CustomType:  customtypes.AAPCustomStringType{},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"skip_tags": schema.StringAttribute{
				Description: "Tags to skip in the workflow job run.",
				Optional:    true,
				Computed:    true,
				CustomType:  customtypes.AAPCustomStringType{},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			// labels is marked as WriteOnly because the value is sent to the API
			// at job launch time but is not returned from the job GET endpoint.
			// The actual labels are managed via the `related` API endpoint.
			"labels": schema.ListAttribute{
				Description: "List of label IDs to apply to the workflow job.",
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

	resp.Diagnostics.Append(data.LaunchWorkflowJobWithResponse(r.client)...)
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
			resp.Diagnostics.AddError("error when waiting for AAP Workflow job to complete", err.Error())
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

	// Create new Workflow Job from workflow job template
	resp.Diagnostics.Append(data.LaunchWorkflowJobWithResponse(r.client)...)
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
			resp.Diagnostics.AddError("error when waiting for AAP Workflow job to complete", err.Error())
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
func (r WorkflowJobResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
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

func (r *WorkflowJobResourceModel) LaunchWorkflowJobWithResponse(client ProviderHTTPClient) diag.Diagnostics {
	body, diags := r.LaunchWorkflowJob(client)
	if diags.HasError() {
		return diags
	}
	return r.ParseHTTPResponse(body)
}

// ParseIgnoredFields parses ignored fields from the AAP API response.
func (r *WorkflowJobResourceModel) ParseIgnoredFields(ignoredFields map[string]interface{}) (diags diag.Diagnostics) {
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
		r.IgnoredFields, diags = types.ListValue(types.StringType, keysList)
	}

	return diags
}
