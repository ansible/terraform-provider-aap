package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource              = &HostResource{}
	_ resource.ResourceWithConfigure = &HostResource{}
)

// NewHostResource is a helper function to simplify the provider implementation
func NewHostResource() resource.Resource {
	return &HostResource{}
}

type HostResourceModelInterface interface {
	ParseHttpResponse(body []byte) error
	CreateRequestBody() ([]byte, diag.Diagnostics)
	GetURL() string
}

// HostResource is the resource implementation.
type HostResource struct {
	client ProviderHTTPClient
}

// Metadata returns the resource type name.
func (r *HostResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_host"
}

// Schema defines the schema for the host resource
func (r *HostResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{

			"inventory_id": schema.Int64Attribute{
				Required: true,
			},
			"instance_id": schema.StringAttribute{
				Required: true,
			},
			"name": schema.StringAttribute{
				Required: true,
			},
			"description": schema.StringAttribute{
				Optional: true,
				Computed: true,
			},
			"host_url": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"variables": schema.StringAttribute{
				Optional:   true,
				CustomType: jsontypes.NormalizedType{},
			},
		},
	}
}

// HostResourceModel maps the resource schema data.
type HostResourceModel struct {
	InventoryId types.Int64          `tfsdk:"inventory_id"`
	InstanceId  types.Int64          `tfsdk:"instance_id"`
	Name        types.String         `tfsdk:"name"`
	Description types.String         `tfsdk:"description"`
	URL         types.String         `tfsdk:"host_url"`
	Variables   jsontypes.Normalized `tfsdk:"variables"`
}

func (d *HostResourceModel) GetURL() string {
	if IsValueProvided(d.URL) {
		return d.URL.ValueString()
	}
	return ""
}

func (d *HostResourceModel) CreateRequestBody() ([]byte, diag.Diagnostics) {
	body := make(map[string]interface{})
	var diags diag.Diagnostics

	// Inventory id
	body["inventory"] = d.InventoryId.ValueInt64()

	// Instance id
	body["instance_id"] = d.InstanceId.ValueInt64()

	// Name
	body["name"] = d.Name.ValueString()

	// Variables
	if IsValueProvided(d.Variables) {
		// var vars map[string]interface{}
		// diags.Append(d.Variables.Unmarshal(&vars)...)
		body["variables"] = d.Variables.ValueString()
	}

	// URL
	if IsValueProvided(d.URL) {
		body["url"] = d.URL.ValueString()
	}

	// Description
	if IsValueProvided(d.Description) {
		body["description"] = d.Description.ValueString()
	}

	json_raw, err := json.Marshal(body)
	if err != nil {
		diags.Append(diag.NewErrorDiagnostic("Body JSON Marshal Error", err.Error()))
		return nil, diags
	}
	return json_raw, diags
}

func (d *HostResourceModel) ParseHttpResponse(body []byte) error {
	/* Unmarshal the json string */
	result := make(map[string]interface{})

	err := json.Unmarshal([]byte(body), &result)
	if err != nil {
		return err
	}

	d.Name = types.StringValue(result["name"].(string))
	d.Description = types.StringValue(result["description"].(string))
	d.URL = types.StringValue(result["url"].(string))

	return nil
}

// Configure adds the provider configured client to the resource.
func (d *HostResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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


func (r HostResource) CreateHost(data HostResourceModelInterface) diag.Diagnostics {
	var diags diag.Diagnostics
	var req_data io.Reader = nil
	req_body, diagCreateReq := data.CreateRequestBody()
	diags.Append(diagCreateReq...)
	if diags.HasError() {
		return diags
	}
	if req_body != nil {
		req_data = bytes.NewReader(req_body)
	}

	resp, body, err := r.client.doRequest(http.MethodPost, "/api/v2/hosts/", req_data)
	if err != nil {
		diags.AddError("Body JSON Marshal Error", err.Error())
		return diags
	}
	if resp == nil {
		diags.AddError("Http response Error", "no http response from server")
		return diags
	}
	if resp.StatusCode != http.StatusCreated {
		diags.AddError("Unexpected Http Status code",
			fmt.Sprintf("expected (%d) got (%s)", http.StatusCreated, resp.Status))
		return diags
	}
	err = data.ParseHttpResponse(body)
	if err != nil {
		diags.AddError("error while parsing the json response: ", err.Error())
		return diags
	}
	return diags
}


func (r HostResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data HostResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(r.CreateHost(&data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r HostResource) DeleteHost(data HostResourceModelInterface) diag.Diagnostics {
	var diags diag.Diagnostics

	resp, _, err := r.client.doRequest(http.MethodDelete, data.GetURL(), nil)
	if err != nil {
		diags.AddError("Body JSON Marshal Error", err.Error())
		return diags
	}
	if resp == nil {
		diags.AddError("Http response Error", "no http response from server")
		return diags
	}
	if resp.StatusCode != http.StatusNoContent {
		diags.AddError("Unexpected Http Status code",
			fmt.Sprintf("expected (%d) got (%s)", http.StatusNoContent, resp.Status))
		return diags
	}
	return diags
}


func (r HostResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data HostResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	resp.Diagnostics.Append(r.DeleteHost(&data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r HostResource) UpdateHost(data HostResourceModelInterface) diag.Diagnostics {
	var diags diag.Diagnostics
	var req_data io.Reader = nil
	req_body, diagCreateReq := data.CreateRequestBody()
	diags.Append(diagCreateReq...)
	if diags.HasError() {
		return diags
	}
	if req_body != nil {
		req_data = bytes.NewReader(req_body)
	}
	resp, body, err := r.client.doRequest(http.MethodPut, data.GetURL(), req_data)

	if err != nil {
		diags.AddError("Body JSON Marshal Error", err.Error())
		return diags
	}
	if resp == nil {
		diags.AddError("Http response Error", "no http response from server")
		return diags
	}
	if resp.StatusCode != http.StatusOK {
		diags.AddError("Unexpected Http Status code",
			fmt.Sprintf("expected (%d) got (%s)", http.StatusOK, resp.Status))
		return diags
	}
	err = data.ParseHttpResponse(body)
	if err != nil {
		diags.AddError("error while parsing the json response: ", err.Error())
		return diags
	}
	return diags
}

func (r HostResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data HostResourceModel
	var data_with_URL HostResourceModel

	// Read Terraform plan and state data into the model
	// The URL is generated once the host is created. To update the correct host, we retrieve the state data and append the URL from the state data to the plan data.
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &data_with_URL)...)
	data.URL = data_with_URL.URL

	resp.Diagnostics.Append(r.UpdateHost(&data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r HostResource) ReadHost(data HostResourceModelInterface) diag.Diagnostics {
	var diags diag.Diagnostics
	// Read existing Host
    host_url := data.GetURL()
	resp, body, err := r.client.doRequest(http.MethodGet, host_url, nil)
	if err != nil {
		diags.AddError("Get Error", err.Error())
		return diags
	}
	if resp == nil {
		diags.AddError("Http response Error", "no http response from server")
		return diags
	}
	if resp.StatusCode != http.StatusOK {
		diags.AddError("Unexpected Http Status code",
			fmt.Sprintf("expected (%d) got (%s)", http.StatusOK, resp.Status))
	}

	err = data.ParseHttpResponse(body)
	if err != nil {
		diags.AddError("error while parsing the json response: ", err.Error())
		return diags
	}
	return diags
}

func (r HostResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data HostResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(r.ReadHost(&data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}