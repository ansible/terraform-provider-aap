package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
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
				Optional: true,
				Computed: true,
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
				Optional: true,
			},
			"enabled": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Defaults true.",
			},
			"group_id": schema.Int64Attribute{
				Optional:    true,
				Description: "Set this option to associate an existing group with a host.",
			},
			"disassociate_group": schema.BoolAttribute{
				Optional: true,
				Description: "Set group_id and and disassociate_group options to remove " +
					"the group from a host without deleting the group.",
			},
		},
	}
}

// HostResourceModel maps the resource schema data.
type HostResourceModel struct {
	InventoryId       types.Int64  `tfsdk:"inventory_id"`
	InstanceId        types.String `tfsdk:"instance_id"`
	Name              types.String `tfsdk:"name"`
	Description       types.String `tfsdk:"description"`
	URL               types.String `tfsdk:"host_url"`
	Variables         types.String `tfsdk:"variables"`
	Enabled           types.Bool   `tfsdk:"enabled"`
	GroupId           types.Int64  `tfsdk:"group_id"`
	DisassociateGroup types.Bool   `tfsdk:"disassociate_group"`
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
	body["instance_id"] = d.InstanceId.ValueString()

	// Name
	body["name"] = d.Name.ValueString()

	// Variables
	if IsValueProvided(d.Variables) {
		body["variables"] = d.Variables.ValueString()
	}

	// Groups
	if IsValueProvided(d.GroupId) {
		body["id"] = d.GroupId.ValueInt64()
	}

	// DisassociateGroup
	if IsValueProvided(d.DisassociateGroup) {
		// DisassociateGroup value does not really matter
		// To remove a group from a host you only need to pass this parameter
		// Add it to the body only if set to true
		if d.DisassociateGroup.ValueBool() {
			body["disassociate_group"] = true
		}
	}

	// Description
	if IsValueProvided(d.Description) {
		body["description"] = d.Description.ValueString()
	}

	// Enabled
	if IsValueProvided(d.Enabled) {
		body["enabled"] = d.Enabled.ValueBool()
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

	err := json.Unmarshal(body, &result)
	if err != nil {
		return err
	}

	d.Name = types.StringValue(result["name"].(string))
	d.URL = types.StringValue(result["url"].(string))

	if r, ok := result["instance_id"]; ok {
		d.InstanceId = types.StringValue(r.(string))
	}

	if r, ok := result["inventory"]; ok {
		d.InventoryId = types.Int64Value(int64(r.(float64)))
	}

	if result["description"] != "" {
		d.Description = types.StringValue(result["description"].(string))
	} else {
		d.Description = types.StringNull()
	}

	if result["variables"] != "" {
		d.Variables = types.StringValue(result["variables"].(string))
	} else {
		d.Variables = types.StringNull()
	}

	if r, ok := result["enabled"]; ok && r != nil {
		d.Enabled = basetypes.NewBoolValue(r.(bool))
	}

	return nil
}

// Configure adds the provider configured client to the resource.
func (d *HostResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func MakeReqData(data HostResourceModelInterface) (io.Reader, diag.Diagnostics) {
	var diags diag.Diagnostics
	var req_data io.Reader = nil

	req_body, diagCreateReq := data.CreateRequestBody()
	diags.Append(diagCreateReq...)

	if diags.HasError() {
		return nil, diags
	}

	if req_body != nil {
		req_data = bytes.NewReader(req_body)
	}

	return req_data, diags
}

func (r HostResource) CreateHost(data HostResourceModelInterface) diag.Diagnostics {
	req_data, diags := MakeReqData(data)
	resp, body, err := r.client.doRequest(http.MethodPost, "/api/v2/hosts/", req_data)
	diags.Append(IsResponseValid(resp, err, http.StatusCreated)...)

	err = data.ParseHttpResponse(body)
	if err != nil {
		diags.AddError("error while parsing the json response: ", err.Error())
		return diags
	}

	return diags
}

func (r HostResource) AssociateGroup(data HostResourceModelInterface) diag.Diagnostics {
	req_data, diags := MakeReqData(data)
	resp, _, err := r.client.doRequest(http.MethodPost, data.GetURL()+"/groups/", req_data)
	diags.Append(IsResponseValid(resp, err, http.StatusNoContent)...)

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

	if IsValueProvided((&data).GroupId) {
		resp.Diagnostics.Append(r.AssociateGroup(&data)...)
		if resp.Diagnostics.HasError() {
			return
		}
		// Save updated data into Terraform state
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	}
}

func (r HostResource) DeleteHost(data HostResourceModelInterface) diag.Diagnostics {
	var diags diag.Diagnostics

	resp, _, err := r.client.doRequest(http.MethodDelete, data.GetURL(), nil)
	diags.Append(IsResponseValid(resp, err, http.StatusNoContent)...)

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
	req_data, diags := MakeReqData(data)
	resp, body, err := r.client.doRequest(http.MethodPut, data.GetURL(), req_data)
	diags.Append(IsResponseValid(resp, err, http.StatusOK)...)

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
	// The URL is generated once the host is created.
	// To update the correct host, we retrieve the state data
	// and append the URL from the state data to the plan data.
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &data_with_URL)...)
	data.URL = data_with_URL.URL

	resp.Diagnostics.Append(r.UpdateHost(&data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)

	if IsValueProvided((&data).GroupId) {
		resp.Diagnostics.Append(r.AssociateGroup(&data)...)
		if resp.Diagnostics.HasError() {
			return
		}
		// Save updated data into Terraform state
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	}
}

func (r HostResource) ReadHost(data HostResourceModelInterface) diag.Diagnostics {
	var diags diag.Diagnostics
	// Read existing Host
	host_url := data.GetURL()
	resp, body, err := r.client.doRequest(http.MethodGet, host_url, nil)
	diags.Append(IsResponseValid(resp, err, http.StatusOK)...)

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
