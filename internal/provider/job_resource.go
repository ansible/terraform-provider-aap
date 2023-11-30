package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework-validators/helpers/validatordiag"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &JobResource{}
	_ resource.ResourceWithImportState = &JobResource{}
)

func NewJobResource() resource.Resource {
	return &JobResource{}
}

type JobResourceModelInterface interface {
	ParseHttpResponse(body []byte) error
	CreateRequestBody() (*bytes.Reader, error)
	GetTemplateId() string
	GetURL() string
}

type JobResource struct {
	client ProviderHttpClient
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
		response.Diagnostics.Append(validatordiag.InvalidAttributeValueDiagnostic(
			request.Path,
			v.Description(ctx),
			fmt.Sprintf("Invalid JSON string => [%s]", request.ConfigValue.ValueString()),
		))
		return
	}
}

func AnsibleJsonVarsValidator() validator.String {
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
			"job_type": schema.StringAttribute{
				Computed: true,
			},
			"job_url": schema.StringAttribute{
				Computed: true,
			},
			"status": schema.StringAttribute{
				Computed: true,
			},
			"extra_vars": schema.StringAttribute{
				Optional: true,
				Validators: []validator.String{
					AnsibleJsonVarsValidator(),
				},
			},
			"ignored_fields": schema.ListAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "The list of properties set by the user but ignored on server side.",
			},
		},
	}
}

// jobResourceModel maps the resource schema data.
type jobResourceModel struct {
	TemplateId    types.Int64  `tfsdk:"job_template_id"`
	Type          types.String `tfsdk:"job_type"`
	URL           types.String `tfsdk:"job_url"`
	Status        types.String `tfsdk:"status"`
	InventoryId   types.Int64  `tfsdk:"inventory_id"`
	ExtraVars     types.String `tfsdk:"extra_vars"`
	IgnoredFields types.List   `tfsdk:"ignored_fields"`
}

var key_mapping = map[string]string{
	"inventory":             "inventory",
	"execution_environment": "execution_environment_id",
}

func (d *jobResourceModel) GetTemplateId() string {
	return d.TemplateId.String()
}

func (d *jobResourceModel) GetURL() string {
	if !d.URL.IsNull() && !d.URL.IsUnknown() {
		return d.URL.ValueString()
	}
	return ""
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

func (d *jobResourceModel) CreateRequestBody() (*bytes.Reader, error) {
	body := make(map[string]interface{})

	// Extra vars
	if IsValueProvided(d.ExtraVars) {
		var extra_vars map[string]interface{}
		_ = json.Unmarshal([]byte(d.ExtraVars.ValueString()), &extra_vars)
		body["extra_vars"] = extra_vars
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
	return bytes.NewReader(json_raw), nil
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

func (r JobResource) CreateJob(data JobResourceModelInterface) error {

	// Create new Job from job template
	req_data, err := data.CreateRequestBody()
	if err != nil {
		return err
	}

	var http_code int
	var body []byte
	post_url := "/api/v2/job_templates/" + data.GetTemplateId() + "/launch/"
	if req_data != nil {
		http_code, body, err = r.client.doRequest("POST", post_url, req_data)
	} else {
		http_code, body, err = r.client.doRequest("POST", post_url, nil)
	}

	if err != nil {
		return err
	}
	if http_code != http.StatusCreated {
		return fmt.Errorf("the server returned status code %d while attempting to create Job", http_code)
	}
	err = data.ParseHttpResponse(body)
	if err != nil {
		return fmt.Errorf("error while parsing the json response: " + err.Error())
	}
	return nil
}

func (r JobResource) ReadJob(data JobResourceModelInterface) error {
	// Read existing Job
	jobURL := data.GetURL()
	if len(jobURL) > 0 {
		http_code, body, err := r.client.doRequest("GET", jobURL, nil)
		if err != nil {
			return err
		}

		if http_code != http.StatusOK {
			return fmt.Errorf("the server returned status code %d while attempting to Get from URL %s", http_code, jobURL)
		}

		err = data.ParseHttpResponse(body)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r JobResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data jobResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	err := r.ReadJob(&data)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unexpected Resource Read error",
			err.Error(),
		)
		return
	}
	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r JobResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data jobResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.CreateJob(&data); err != nil {
		resp.Diagnostics.AddError(
			"Unexpected Resource Creation error",
			err.Error(),
		)
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r JobResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
}

func (r JobResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data jobResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	// Create new Job from job template
	if err := r.CreateJob(&data); err != nil {
		resp.Diagnostics.AddError(
			"Unexpected Resource Update error",
			err.Error(),
		)
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *JobResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("job_template_id"), req, resp)
}
