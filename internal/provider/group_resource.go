package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource              = &GroupResource{}
	_ resource.ResourceWithConfigure = &GroupResource{}
)

// NewGroupResource is a helper function to simplify the provider implementation.
func NewGroupResource() resource.Resource {
	return &GroupResource{}
}

type GroupResourceModelInterface interface {
	ParseHttpResponse(body []byte) error
	CreateRequestBody() ([]byte, diag.Diagnostics)
	GetURL() string
}

// GroupResource is the resource implementation.
type GroupResource struct {
	client ProviderHTTPClient
}

// Metadata returns the resource type name.
func (r *GroupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group"
}

// Schema defines the schema for the group resource.
func (r *GroupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
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
			},
			"group_url": schema.StringAttribute{
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

// GroupResourceModel maps the resource schema data.
type GroupResourceModel struct {
	InventoryId types.Int64          `tfsdk:"inventory_id"`
	Name        types.String         `tfsdk:"name"`
	Description types.String         `tfsdk:"description"`
	URL         types.String         `tfsdk:"group_url"`
	Variables   jsontypes.Normalized `tfsdk:"variables"`
}

func IsValueProvided(value attr.Value) bool {
	return !value.IsNull() && !value.IsUnknown()
}

func (d *GroupResourceModel) GetURL() string {
	if IsValueProvided(d.URL) {
		return d.URL.ValueString()
	}
	return ""
}

func (d *GroupResourceModel) CreateRequestBody() ([]byte, diag.Diagnostics) {
	body := make(map[string]interface{})
	var diags diag.Diagnostics

	// Inventory id
	body["inventory"] = d.InventoryId.ValueInt64()

	// Name
	body["name"] = d.Name.ValueString()

	// Variables
	if IsValueProvided(d.Variables) {
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

func (d *GroupResourceModel) ParseHttpResponse(body []byte) error {
	/* Unmarshal the json string */
	result := make(map[string]interface{})

	err := json.Unmarshal([]byte(body), &result)
	if err != nil {
		return err
	}

	d.Name = types.StringValue(result["name"].(string))
	if result["description"] != "" {
		d.Description = types.StringValue(result["description"].(string))
	} else {
		d.Description = types.StringNull()
	}
	d.URL = types.StringValue(result["url"].(string))
	if result["variables"] != nil {
		d.Variables = jsontypes.NewNormalizedValue(result["variables"].(string))
	} else {
		d.Variables = jsontypes.NewNormalizedNull()
	}

	return nil
}

// Configure adds the provider configured client to the resource.
func (d *GroupResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r GroupResource) CreateGroup(data GroupResourceModelInterface) diag.Diagnostics {
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

	resp, body, err := r.client.doRequest(http.MethodPost, "/api/v2/groups/", req_data)
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

func (r GroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data GroupResourceModel

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

func (r GroupResource) DeleteGroup(data GroupResourceModelInterface) diag.Diagnostics {
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

func (r GroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data GroupResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	resp.Diagnostics.Append(r.DeleteGroup(&data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r GroupResource) UpdateGroup(data GroupResourceModelInterface) diag.Diagnostics {
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

func (r GroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data GroupResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(r.UpdateGroup(&data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r GroupResource) ReadGroup(data GroupResourceModelInterface) diag.Diagnostics {
	var diags diag.Diagnostics
	// Read existing Group
	group_url := data.GetURL()
	resp, body, err := r.client.doRequest(http.MethodGet, group_url, nil)
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

func (r GroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data GroupResourceModel

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
