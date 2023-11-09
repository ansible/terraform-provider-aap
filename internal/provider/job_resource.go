package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/helpers/validatordiag"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &JobResource{}
	_ resource.ResourceWithImportState = &JobResource{}
)

func NewJobResource() resource.Resource {
	return &JobResource{}
}

// inventoryDataSource is the data source implementation.
type JobResource struct {
	client *AAPClient
}

// Metadata returns the resource type name.
func (r *JobResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_job"
}

var _ validator.String = ansibleVarsValidator{}

// ansibleVarsValidator validates that a string Attribute's is a valid JSON.
type ansibleVarsValidator struct{}

// Description describes the validation in plain text formatting.
func (validator ansibleVarsValidator) Description(_ context.Context) string {
	return "string must be a valid JSON."
}

// MarkdownDescription describes the validation in Markdown formatting.
func (validator ansibleVarsValidator) MarkdownDescription(ctx context.Context) string {
	return validator.Description(ctx)
}

// Validate performs the validation.
func (v ansibleVarsValidator) ValidateString(ctx context.Context, request validator.StringRequest, response *validator.StringResponse) {
	if request.ConfigValue.IsNull() || request.ConfigValue.IsUnknown() {
		return
	}
	if !json.Valid([]byte(request.ConfigValue.ValueString())) {
		response.Diagnostics.Append(validatordiag.InvalidAttributeValueLengthDiagnostic(
			request.Path,
			v.Description(ctx),
			fmt.Sprintf("Invalid JSON string => [%s]", request.ConfigValue.ValueString()),
		))
		return
	}
}

func ansible_json_vars_validator() validator.String {
	return ansibleVarsValidator{}
}

// Schema defines the schema for the resource.
func (d *JobResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"job_template_id": schema.Int64Attribute{
				Required: true,
			},
			"inventory_id": schema.Int64Attribute{
				Optional: true,
			},
			"execution_environment_id": schema.Int64Attribute{
				Optional: true,
			},
			"job_type": schema.StringAttribute{
				Computed: true,
			},
			"job_url": schema.StringAttribute{
				Computed: true,
			},
			"status": schema.StringAttribute{
				Computed: true,
			},
			"wait_for_completion": schema.BoolAttribute{
				Optional: true,
			},
			"wait_duration": schema.Int64Attribute{
				Optional: true,
			},
			"extra_vars": schema.StringAttribute{
				Optional: true,
				Validators: []validator.String{
					ansible_json_vars_validator(),
				},
			},
			"ignored_fields": schema.ListAttribute{
				ElementType: types.StringType,
				Computed:    true,
			},
		},
	}
}

// jobResourceModel maps the resource schema data.
type jobResourceModel struct {
	Id                   types.Int64  `tfsdk:"job_template_id"`
	Type                 types.String `tfsdk:"job_type"`
	URL                  types.String `tfsdk:"job_url"`
	Status               types.String `tfsdk:"status"`
	WaitForCompletion    types.Bool   `tfsdk:"wait_for_completion"`
	WaitDuration         types.Int64  `tfsdk:"wait_duration"`
	InventoryId          types.Int64  `tfsdk:"inventory_id"`
	ExecutionEnvironment types.Int64  `tfsdk:"execution_environment_id"`
	ExtraVars            types.String `tfsdk:"extra_vars"`
	IgnoredFields        types.List   `tfsdk:"ignored_fields"`
}

var key_mapping = map[string]string{
	"inventory":             "inventory",
	"execution_environment": "execution_environment_id",
}

func (d *jobResourceModel) ParseHttpResponse(body []byte) error {
	/* Unmarshal the json string */
	var result map[string]interface{}
	err := json.Unmarshal(body, &result)
	if err != nil {
		return err
	}

	d.Type = types.StringValue(result["job_type"].(string))
	d.URL = types.StringValue(result["url"].(string))
	d.Status = types.StringValue(result["status"].(string))
	d.IgnoredFields = types.ListNull(types.StringType)

	if value, ok := result["ignored_fields"]; ok {
		var keys_list []attr.Value = []attr.Value{}
		for k := range value.(map[string]interface{}) {
			key := k
			if v, ok := key_mapping[k]; ok {
				key = v
			}
			keys_list = append(keys_list, types.StringValue(key))
		}
		if len(keys_list) > 0 {
			d.IgnoredFields, _ = types.ListValue(types.StringType, keys_list)
		}
	}

	return nil
}

func IsValueProvided(value attr.Value) bool {
	return !value.IsNull() && !value.IsUnknown()
}

func (d *jobResourceModel) CreateRequestBody(ctx context.Context) (*bytes.Reader, error) {
	body := make(map[string]interface{})

	// Extra vars
	if IsValueProvided(d.ExtraVars) {
		var extra_vars map[string]interface{}
		_ = json.Unmarshal([]byte(d.ExtraVars.ValueString()), &extra_vars)
		body["extra_vars"] = extra_vars
	}

	// Execution environment
	if IsValueProvided(d.ExecutionEnvironment) {
		body["execution_environment"] = d.ExecutionEnvironment.ValueInt64()
	}

	// Inventory
	if IsValueProvided(d.InventoryId) {
		body["inventory"] = d.InventoryId.ValueInt64()
	}

	if len(body) == 0 {
		return nil, nil
	}
	json_raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	tflog.Info(ctx, fmt.Sprintf("Sending request with data %s", string(json_raw)))
	return bytes.NewReader(json_raw), nil
}

func (r *JobResource) WaitForJob(job_url string, statuses []string, wait_duration int64) (string, error) {
	// wait until the resource reachs one of the expected statuses
	start := time.Now()
	var last_error error
	for {
		time.Sleep(5 * time.Second)
		// Read Job
		http_code, body, err := r.client.doRequest("GET", job_url)
		if err != nil {
			last_error = err
		}
		if http_code == http.StatusOK {
			var result map[string]interface{}
			err = json.Unmarshal(body, &result)
			if err != nil {
				last_error = err
			} else {
				job_status := result["status"].(string)
				if slices.Contains(statuses, job_status) {
					return job_status, nil
				}
			}
		}
		if start.Sub(time.Now()).Seconds() >= float64(wait_duration) {
			if last_error == nil {
				last_error = fmt.Errorf("The resource did not reached the expected status.")
			}
			return "", last_error
		}
	}
}

// Configure adds the provider configured client to the data source.
func (d *JobResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	d.client = client
}

func (r JobResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data jobResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if !data.URL.IsNull() && !data.URL.IsUnknown() {
		// Read existing Job
		http_code, body, err := r.client.doRequest("GET", data.URL.ValueString())
		if err != nil {
			resp.Diagnostics.AddError(
				"Unexpected Resource Read error",
				err.Error(),
			)
			return
		}

		if http_code != http.StatusOK {
			resp.Diagnostics.AddError(
				"Unexpected Resource Read error",
				fmt.Sprintf("The server returned status code %d while attempting to Get from URL %s", http_code, data.URL.ValueString()),
			)
			return
		}

		tflog.Info(ctx, fmt.Sprintf("HTTP GET [%s] => %s", data.URL.ValueString(), string(body)))
		err = data.ParseHttpResponse(body)
		if err != nil {
			resp.Diagnostics.AddError(
				"Unexpected Resource Read error",
				"Failed to parse HTTP JSON Response: "+err.Error(),
			)
			return
		}
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r JobResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *jobResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Create new Job from job template
	req_data, err := data.CreateRequestBody(ctx)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unexpected Resource Creation error",
			err.Error(),
		)
	}
	var http_code int
	var body []byte
	post_url := "/api/v2/job_templates/" + data.Id.String() + "/launch/"
	if req_data != nil {
		http_code, body, err = r.client.doRequestWithBody("POST", post_url, req_data)
	} else {
		http_code, body, err = r.client.doRequest("POST", post_url)
	}

	if err != nil {
		resp.Diagnostics.AddError(
			"Unexpected Resource Creation error",
			err.Error(),
		)
		return
	}

	if http_code != http.StatusCreated {
		resp.Diagnostics.AddError(
			"Unexpected Resource Creation error",
			fmt.Sprintf("The server returned status code %d while attempting to create Job", http_code),
		)
		return
	}

	tflog.Info(ctx, fmt.Sprintf("HTTP POST [%s] => %s", post_url, string(body)))
	err = data.ParseHttpResponse(body)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unexpected Resource Creation error",
			"Failed to parse HTTP JSON Response: "+err.Error(),
		)
		return
	}

	if data.WaitForCompletion.ValueBool() {
		var wait_duration int64 = 120
		if !(data.WaitDuration.IsNull() && data.WaitDuration.IsUnknown()) {
			wait_duration = data.WaitDuration.ValueInt64()
		}
		job_status, err := r.WaitForJob(data.URL.ValueString(), []string{"successful", "failed"}, wait_duration)
		if err != nil {
			resp.Diagnostics.AddError(
				"Unexpected Resource Creation error",
				"An error occurred while waiting for Job to complete: "+err.Error(),
			)
			return
		}
		data.Status = types.StringValue(job_status)
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r JobResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data jobResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if data.URL.IsNull() || data.URL.IsUnknown() {
		return
	}

	// Read Job
	http_status_code, body, err := r.client.doRequest("GET", data.URL.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Unexpected Resource Deletion error",
			err.Error(),
		)
		return
	}
	if http_status_code == http.StatusNotFound {
		// the job does not exist, we can exit here
		return
	}
	if http_status_code != http.StatusOK {
		// Unexpected http status code
		resp.Diagnostics.AddError(
			"Unexpected Resource Deletion error",
			fmt.Sprintf("The server returned an unexpected http status code %d while trying to read from path %s", http_status_code, data.URL.ValueString()),
		)
		return
	}

	tflog.Info(ctx, fmt.Sprintf("HTTP GET [%s] => %s", data.URL.ValueString(), string(body)))
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		resp.Diagnostics.AddError(
			"Unexpected Resource Deletion error",
			"Failed to parse HTTP JSON response: "+err.Error(),
		)
		return
	}

	// Ensure we can cancel the job prior the deletion
	job_status := result["status"].(string)
	cancel_url := result["related"].(map[string]interface{})["cancel"].(string)
	http_status_code, body, err = r.client.doRequest("GET", cancel_url)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unexpected Resource Deletion error",
			"Unable to read job cancel from url: "+cancel_url+" - "+err.Error(),
		)
		return
	}
	if http_status_code == http.StatusOK {
		tflog.Info(ctx, fmt.Sprintf("HTTP GET [%s] => %s", cancel_url, string(body)))
		if err := json.Unmarshal(body, &result); err != nil {
			resp.Diagnostics.AddError(
				"Unexpected Resource Deletion error",
				"Unable to parse JSON HTTP response: "+err.Error(),
			)
			return
		}
		if result["can_cancel"].(bool) {
			tflog.Info(ctx, fmt.Sprintf("Cancel job prior the deletion, the current status is '%s'", job_status))
			// cancel job before deletion
			_, _, err = r.client.doRequest("POST", cancel_url)
			if err != nil {
				resp.Diagnostics.AddError(
					"Unexpected Resource Deletion error",
					err.Error(),
				)
				return
			}
			// wait until the job is canceled
			_, err := r.WaitForJob(data.URL.ValueString(), []string{"canceled"}, 20)
			if err != nil {
				resp.Diagnostics.AddError(
					"Unexpected Resource Deletion error",
					"An error occurred while waiting for Job to be canceled: "+err.Error(),
				)
				return
			}
		}
	}

	// Delete Job
	http_status_code, _, _ = r.client.doRequest("DELETE", data.URL.ValueString())
	if http_status_code != http.StatusNotFound && http_status_code != http.StatusNoContent {
		resp.Diagnostics.AddError(
			"Unexpected Resource Deletion error",
			fmt.Sprintf("The server returned an unexpected http status code %d while trying to delete from path %s", http_status_code, data.URL.ValueString()),
		)
		return
	}
}

func (r JobResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data jobResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	// Create new Job from job template
	req_data, err := data.CreateRequestBody(ctx)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unexpected Resource Creation error",
			err.Error(),
		)
	}
	var http_code int
	var body []byte
	post_url := "/api/v2/job_templates/" + data.Id.String() + "/launch/"
	if req_data != nil {
		http_code, body, err = r.client.doRequestWithBody("POST", post_url, req_data)
	} else {
		http_code, body, err = r.client.doRequest("POST", post_url)
	}

	if err != nil {
		resp.Diagnostics.AddError(
			"Unexpected Resource Update error",
			err.Error(),
		)
		return
	}

	if http_code != http.StatusCreated {
		resp.Diagnostics.AddError(
			"Unexpected Resource Update error",
			fmt.Sprintf("The server returned status code %d while attempting to create Job", http_code),
		)
		return
	}

	tflog.Info(ctx, fmt.Sprintf("HTTP POST [%s] => %s", post_url, string(body)))
	err = data.ParseHttpResponse(body)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unexpected Resource Update error",
			"Failed to parse HTTP JSON Response: "+err.Error(),
		)
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *JobResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("job_template_id"), req, resp)
}
