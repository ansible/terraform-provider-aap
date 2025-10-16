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
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
)

// Default value for the wait_for_completion timeout, so the linter doesn't complain.
const waitForCompletionTimeoutDefault int64 = 120

// Job AAP API model
type JobAPIModel struct {
	TemplateID    int64                  `json:"job_template,omitempty"`
	Type          string                 `json:"job_type,omitempty"`
	URL           string                 `json:"url,omitempty"`
	Status        string                 `json:"status,omitempty"`
	Inventory     int64                  `json:"inventory,omitempty"`
	ExtraVars     string                 `json:"extra_vars,omitempty"`
	IgnoredFields map[string]interface{} `json:"ignored_fields,omitempty"`
}

// JobModel are the attributes that are provided by the user and also used by the action.
type JobModel struct {
	TemplateID               types.Int64                      `tfsdk:"job_template_id"`
	InventoryID              types.Int64                      `tfsdk:"inventory_id"`
	ExtraVars                customtypes.AAPCustomStringValue `tfsdk:"extra_vars"`
	WaitForCompletion        types.Bool                       `tfsdk:"wait_for_completion"`
	WaitForCompletionTimeout types.Int64                      `tfsdk:"wait_for_completion_timeout_seconds"`
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

// Given a string with the name of an AAP Job state, this function returns `true`
// if such state is final and cannot transition further; a.k.a, the job is completed.
func IsFinalStateAAPJob(state string) bool {
	finalStates := map[string]bool{
		"new":        false,
		"pending":    false,
		"waiting":    false,
		"running":    false,
		"successful": true,
		"failed":     true,
		"error":      true,
		"canceled":   true,
	}
	result, isPresent := finalStates[state]
	return isPresent && result
}

func retryUntilAAPJobReachesAnyFinalState(ctx context.Context, client ProviderHTTPClient, url string, status *string) retry.RetryFunc {
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
				Description: "Id of the job template.",
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

func (r *JobResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data JobResourceModel

	// Read Terraform plan data into job resource model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(data.LaunchJobWithResponse(r.client)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If the job was configured to wait for completion, start polling the job status
	// and wait for it to complete before marking the resource as created
	if data.WaitForCompletion.ValueBool() {
		timeout := time.Duration(data.WaitForCompletionTimeout.ValueInt64()) * time.Second
		var status string
		err := retry.RetryContext(ctx, timeout, retryUntilAAPJobReachesAnyFinalState(ctx, r.client, data.URL.ValueString(), &status))
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
	diags = data.ParseHttpResponse(readResponseBody)
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

func (r *JobResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data JobResourceModel

	// Read Terraform plan data into job resource model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

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
		err := retry.RetryContext(ctx, timeout, retryUntilAAPJobReachesAnyFinalState(ctx, r.client, data.URL.ValueString(), &status))
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

// CreateRequestBody creates a JSON encoded request body from the job resource data
func (r *JobModel) CreateRequestBody() ([]byte, diag.Diagnostics) {
	var diags diag.Diagnostics
	var inventoryID int64

	// Use default inventory if not provided
	if r.InventoryID.ValueInt64() != 0 {
		inventoryID = r.InventoryID.ValueInt64()
	}

	// Convert job resource data to API data model
	job := JobAPIModel{
		ExtraVars: r.ExtraVars.ValueString(),
		Inventory: inventoryID,
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

// ParseHttpResponse updates the job resource data from an AAP API response
func (r *JobResourceModel) ParseHttpResponse(body []byte) diag.Diagnostics {
	var diags diag.Diagnostics

	// Unmarshal the JSON response
	var resultApiJob JobAPIModel
	err := json.Unmarshal(body, &resultApiJob)
	if err != nil {
		diags.AddError("Error parsing JSON response from AAP", err.Error())
		return diags
	}

	// Map response to the job resource schema and update attribute values
	r.Type = types.StringValue(resultApiJob.Type)
	r.URL = types.StringValue(resultApiJob.URL)
	r.Status = types.StringValue(resultApiJob.Status)
	r.TemplateID = types.Int64Value(resultApiJob.TemplateID)
	r.InventoryID = types.Int64Value(resultApiJob.Inventory)
	diags = r.ParseIgnoredFields(resultApiJob.IgnoredFields)
	return diags
}

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

func (r *JobModel) LaunchJob(client ProviderHTTPClient) (body []byte, diags diag.Diagnostics) {
	// Create request body from job data
	requestBody, diagCreateReq := r.CreateRequestBody()
	diags.Append(diagCreateReq...)
	if diags.HasError() {
		return nil, diags
	}

	requestData := bytes.NewReader(requestBody)
	var postURL = path.Join(client.getApiEndpoint(), "job_templates", r.TemplateID.String(), "launch")
	resp, body, err := client.doRequest(http.MethodPost, postURL, nil, requestData)
	diags.Append(ValidateResponse(resp, body, err, []int{http.StatusCreated})...)
	if diags.HasError() {
		return nil, diags
	}

	return body, diags
}

func (r *JobResourceModel) LaunchJobWithResponse(client ProviderHTTPClient) diag.Diagnostics {
	body, diags := r.LaunchJob(client)
	if diags.HasError() {
		return diags
	}
	return r.ParseHttpResponse(body)
}
