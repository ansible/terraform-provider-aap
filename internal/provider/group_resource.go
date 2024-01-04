package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource = &groupResource{}
)

// NewGroupResource is a helper function to simplify the provider implementation.
func NewGroupResource() resource.Resource {
	return &groupResource{}
}

type GroupResourceModelInterface interface {
	ParseHttpResponse(body []byte) error
	CreateRequestBody() ([]byte, diag.Diagnostics)
	GetURL() string
}

// groupResource is the resource implementation.
type groupResource struct {
	client ProviderHTTPClient
}

// Metadata returns the resource type name.
func (r *groupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group"
}

// Schema defines the schema for the group resource.
func (r *groupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{

			"inventory_id": schema.Int64Attribute{
				Required: true,
			},
			"name": schema.StringAttribute{
				Required: true,
			},
			"description": schema.StringAttribute{
				Optional: true,
				Computed: true,
			},
			"group_url": schema.StringAttribute{
				Computed: true,
			},
			"variables": schema.StringAttribute{
				Optional:   true,
				CustomType: jsontypes.NormalizedType{},
			},
		},
	}
}

// groupResourceModel maps the resource schema data.
type groupResourceModel struct {
	InventoryId types.Int64          `tfsdk:"inventory_id"`
	Name        types.String         `tfsdk:"name"`
	Description types.String         `tfsdk:"description"`
	URL         types.String         `tfsdk:"group_url"`
	Variables   jsontypes.Normalized `tfsdk:"variables"`
}

func IsValueProvided(value attr.Value) bool {
	return !value.IsNull() && !value.IsUnknown()
}

func (d *groupResourceModel) GetURL() string {
	if !d.URL.IsNull() && !d.URL.IsUnknown() {
		return d.URL.ValueString()
	}
	return ""
}

func GetFunctionName(caller int) string {
	pc, _, _, result := runtime.Caller(caller + 1)
	if !result {
		return ""
	}
	f := runtime.FuncForPC(pc)
	if f == nil {
		return ""
	}
	return f.Name()
}

func (d *groupResourceModel) CreateRequestBody() ([]byte, diag.Diagnostics) {
	body := make(map[string]interface{})
	var diags diag.Diagnostics

	// Inventory id
	body["inventory"] = d.InventoryId.ValueInt64()

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

	if len(body) == 0 {
		return nil, diags
	}
	json_raw, err := json.Marshal(body)
	if err != nil {
		diags.Append(diag.NewErrorDiagnostic("Body JSON Marshal Error", err.Error()))
		return nil, diags
	}
	return json_raw, diags
}

func (d *groupResourceModel) ParseHttpResponse(body []byte) error {
	/* Unmarshal the json string */
	result := make(map[string]interface{})
	// replacer := strings.NewReplacer("\r", "", "\n", "")
	// data := replacer.Replace(string(body))
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
func (d *groupResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r groupResource) CreateGroup(data GroupResourceModelInterface) diag.Diagnostics {
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

	post_url := "/api/v2/groups/"
	resp, body, err := r.client.doRequest(http.MethodPost, post_url, req_data)
	if err != nil {
		diags.AddError(GetFunctionName(0)+" Body JSON Marshal Error", err.Error())
		return diags
	}
	if resp == nil {
		diags.AddError(GetFunctionName(0)+" Http response Error", "no http response from server")
		return diags
	}
	if resp.StatusCode != http.StatusCreated {
		diags.AddError(GetFunctionName(0)+" Unexpected Http Status code",
			fmt.Sprintf("expected (%d) got (%s)", http.StatusCreated, resp.Status))
		return diags
	}
	err = data.ParseHttpResponse(body)
	if err != nil {
		diags.AddError(GetFunctionName(0)+" error while parsing the json response: ", err.Error())
		return diags
	}
	return diags
}

func (r groupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data groupResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(r.CreateGroup(&data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r groupResource) DeleteGroup(data GroupResourceModelInterface) diag.Diagnostics {
	var diags diag.Diagnostics
	group_url := data.GetURL()

	resp, _, err := r.client.doRequest(http.MethodDelete, group_url, nil)
	if err != nil {
		diags.AddError(GetFunctionName(0)+" Body JSON Marshal Error", err.Error())
		return diags
	}
	if resp == nil {
		diags.AddError(GetFunctionName(0)+" Http response Error", "no http response from server")
		return diags
	}
	if resp.StatusCode != http.StatusNoContent {
		diags.AddError(GetFunctionName(0)+" Unexpected Http Status code",
			fmt.Sprintf("expected (%d) got (%s)", http.StatusNoContent, resp.Status))
		return diags
	}
	return diags
}

func (r groupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data groupResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	resp.Diagnostics.Append(r.DeleteGroup(&data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r groupResource) UpdateGroup(data GroupResourceModelInterface) diag.Diagnostics {
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
		diags.AddError(GetFunctionName(0)+" Body JSON Marshal Error", err.Error())
		return diags
	}
	if resp == nil {
		diags.AddError(GetFunctionName(0)+" Http response Error", "no http response from server")
		return diags
	}
	if resp.StatusCode != http.StatusOK {
		diags.AddError(GetFunctionName(0)+" Unexpected Http Status code",
			fmt.Sprintf("expected (%d) got (%s)", http.StatusOK, resp.Status))
		return diags
	}
	err = data.ParseHttpResponse(body)
	if err != nil {
		diags.AddError(GetFunctionName(0)+" error while parsing the json response: ", err.Error())
		return diags
	}
	return diags
}

func (r groupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data groupResourceModel
	var data_with_URL groupResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &data_with_URL)...)
	data.URL = data_with_URL.URL

	resp.Diagnostics.Append(r.UpdateGroup(&data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r groupResource) ReadGroup(data GroupResourceModelInterface) diag.Diagnostics {
	var diags diag.Diagnostics
	// Read existing Group
	group_url := data.GetURL()
	resp, body, err := r.client.doRequest(http.MethodGet, group_url, nil)
	if err != nil {
		diags.AddError(GetFunctionName(0)+" Get Error", err.Error())
		return diags
	}
	if resp == nil {
		diags.AddError(GetFunctionName(0)+" Http response Error", "no http response from server")
		return diags
	}
	if resp.StatusCode != http.StatusOK {
		diags.AddError(GetFunctionName(0)+" Unexpected Http Status code",
			fmt.Sprintf("expected (%d) got (%s)", http.StatusOK, resp.Status))
	}

	err = data.ParseHttpResponse(body)
	if err != nil {
		diags.AddError(GetFunctionName(0)+" error while parsing the json response: ", err.Error())
		return diags
	}
	return diags
}

func (r groupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data groupResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(r.ReadGroup(&data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
