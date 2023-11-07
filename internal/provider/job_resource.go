package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
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

// Schema defines the schema for the resource.
func (d *JobResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"job_template_id": schema.Int64Attribute{
				Required: true,
			},
			"job_type": schema.StringAttribute{
				Computed: true,
				Optional: true,
			},
			"job_url": schema.StringAttribute{
				Computed: true,
				Optional: true,
			},
			"status": schema.StringAttribute{
				Optional: true,
				Computed: true,
			},
			"wait_for_completion": schema.BoolAttribute{
				Optional: true,
			},
			"wait_duration": schema.Int64Attribute{
				Optional: true,
			},
		},
	}
}

// jobResourceModel maps the resource schema data.
type jobResourceModel struct {
	Id                types.Int64  `tfsdk:"job_template_id"`
	Type              types.String `tfsdk:"job_type"`
	URL               types.String `tfsdk:"job_url"`
	Status            types.String `tfsdk:"status"`
	WaitForCompletion types.Bool   `tfsdk:"wait_for_completion"`
	WaitDuration      types.Int64  `tfsdk:"wait_duration"`
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

	if !data.URL.IsNull() {
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

		/* Unmarshal the json string */
		var result map[string]interface{}
		err = json.Unmarshal(body, &result)
		if err != nil {
			resp.Diagnostics.AddError(
				"Unexpected Resource Read error",
				"Failed to parse HTTP JSON Response: "+err.Error(),
			)
			return
		}

		data.Type = types.StringValue(result["job_type"].(string))
		data.Status = types.StringValue(result["status"].(string))
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
	http_code, body, err := r.client.doRequest("POST", "/api/v2/job_templates/"+data.Id.String()+"/launch/")
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

	/* Unmarshal the json string */
	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unexpected Resource Creation error",
			"Failed to parse HTTP JSON Response: "+err.Error(),
		)
		return
	}

	data.Type = types.StringValue(result["job_type"].(string))
	data.URL = types.StringValue(result["url"].(string))
	data.Status = types.StringValue(result["status"].(string))

	if data.WaitForCompletion.ValueBool() {
		var wait_duration int64 = 120
		if !data.WaitDuration.IsNull() {
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

	if data.URL.IsNull() {
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

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(body), &result); err != nil {
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
		if err := json.Unmarshal([]byte(body), &result); err != nil {
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
	http_code, body, err := r.client.doRequest("POST", "/api/v2/job_templates/"+data.Id.String()+"/launch/")
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

	/* Unmarshal the json string */
	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unexpected Resource Update error",
			"Failed to parse HTTP JSON Response: "+err.Error(),
		)
		return
	}

	data.Type = types.StringValue(result["job_type"].(string))
	data.URL = types.StringValue(result["url"].(string))
	data.Status = types.StringValue(result["status"].(string))

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *JobResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("job_template_id"), req, resp)
}
