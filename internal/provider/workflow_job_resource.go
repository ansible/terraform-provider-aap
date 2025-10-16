package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path"

	"github.com/ansible/terraform-provider-aap/internal/provider/customtypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// WorkflowJobAPIModel represents the AAP API model for workflow jobs.
type WorkflowJobAPIModel struct {
	TemplateID    int64                  `json:"workflow_job_template,omitempty"`
	Inventory     int64                  `json:"inventory,omitempty"`
	Type          string                 `json:"job_type,omitempty"`
	URL           string                 `json:"url,omitempty"`
	Status        string                 `json:"status,omitempty"`
	ExtraVars     string                 `json:"extra_vars,omitempty"`
	IgnoredFields map[string]interface{} `json:"ignored_fields,omitempty"`
}

type WorkflowJobModel struct {
	TemplateID  types.Int64                      `tfsdk:"workflow_job_template_id"`
	InventoryID types.Int64                      `tfsdk:"inventory_id"`
	ExtraVars   customtypes.AAPCustomStringValue `tfsdk:"extra_vars"`
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

	resp.Diagnostics.Append(data.LaunchWorkflowJobWithResponse(r.client)...)
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

	// Create new Workflow Job from workflow job template
	resp.Diagnostics.Append(data.LaunchWorkflowJobWithResponse(r.client)...)
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

// CreateRequestBody creates a JSON encoded request body from the workflow job resource data
func (r *WorkflowJobModel) CreateRequestBody() ([]byte, diag.Diagnostics) {
	var diags diag.Diagnostics

	// Convert workflow job resource data to API data model
	workflowJob := WorkflowJobAPIModel{
		ExtraVars: r.ExtraVars.ValueString(),
	}

	// Set inventory id if provided
	if r.InventoryID.ValueInt64() != 0 {
		workflowJob.Inventory = r.InventoryID.ValueInt64()
	}

	// Create JSON encoded request body
	jsonBody, err := json.Marshal(workflowJob)
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

	// Map response to the job resource schema and update attribute values
	r.Type = types.StringValue(resultAPIWorkflowJob.Type)
	r.URL = types.StringValue(resultAPIWorkflowJob.URL)
	r.Status = types.StringValue(resultAPIWorkflowJob.Status)
	r.TemplateID = types.Int64Value(resultAPIWorkflowJob.TemplateID)
	r.InventoryID = types.Int64Value(resultAPIWorkflowJob.Inventory)
	diags = r.ParseIgnoredFields(resultAPIWorkflowJob.IgnoredFields)
	return diags
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

func (r *WorkflowJobModel) LaunchWorkflowJob(client ProviderHTTPClient) ([]byte, diag.Diagnostics) {
	// Create new Workflow Job from workflow job template
	var diags diag.Diagnostics

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

func (r *WorkflowJobResourceModel) LaunchWorkflowJobWithResponse(client ProviderHTTPClient) diag.Diagnostics {
	body, diags := r.LaunchWorkflowJob(client)
	if diags.HasError() {
		return diags
	}
	return r.ParseHTTPResponse(body)
}
