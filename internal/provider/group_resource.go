package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path"

	"github.com/ansible/terraform-provider-aap/internal/provider/customtypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Group AAP API model
type GroupAPIModel struct {
	InventoryId int64  `json:"inventory"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	URL         string `json:"url,omitempty"`
	Variables   string `json:"variables,omitempty"`
	Id          int64  `json:"id,omitempty"`
}

// GroupResourceModel maps the group resource schema to a Go struct
type GroupResourceModel struct {
	InventoryId types.Int64                      `tfsdk:"inventory_id"`
	Name        types.String                     `tfsdk:"name"`
	Description types.String                     `tfsdk:"description"`
	URL         types.String                     `tfsdk:"url"`
	Variables   customtypes.AAPCustomStringValue `tfsdk:"variables"`
	Id          types.Int64                      `tfsdk:"id"`
}

// GroupResource is the resource implementation.
type GroupResource struct {
	client ProviderHTTPClient
}

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource              = &GroupResource{}
	_ resource.ResourceWithConfigure = &GroupResource{}
)

// NewGroupResource is a helper function to simplify the provider implementation.
func NewGroupResource() resource.Resource {
	return &GroupResource{}
}

// Metadata returns the resource type name.
func (r *GroupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group"
}

// Configure adds the provider configured client to the resource
func (r *GroupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// Schema defines the schema for the group resource.
func (r *GroupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"inventory_id": schema.Int64Attribute{
				Required:    true,
				Description: "Inventory id",
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the group",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Description for the group",
			},
			"url": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Description: "URL for the group",
			},
			"id": schema.Int64Attribute{
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
				Description: "Group Id",
			},
			"variables": schema.StringAttribute{
				Description: "Variables for the group configuration. Must be provided as either a JSON or YAML string.",
				Optional:    true,
				CustomType:  customtypes.AAPCustomStringType{},
			},
		},
		Description: `Creates an inventory group.`,
	}
}

// Create creates the group resource and sets the Terraform state on success.
func (r *GroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data GroupResourceModel
	var diags diag.Diagnostics

	// Read Terraform plan data into group resource model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Create request body from group data
	createRequestBody, diags := data.CreateRequestBody()
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	requestData := bytes.NewReader(createRequestBody)

	// Create new group in AAP
	groupsURL := path.Join(r.client.getApiEndpoint(), "groups")
	createResponseBody, diags := r.client.Create(groupsURL, requestData)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save new group data into group resource model
	diags = data.ParseHttpResponse(createResponseBody)
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

// Read refreshes the Terraform state with the latest group data.
func (r *GroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data GroupResourceModel
	var diags diag.Diagnostics

	// Read current Terraform state data into group resource model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get latest group data from AAP
	readResponseBody, diags := r.client.Get(data.URL.ValueString())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save latest group data into group resource model
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

// Update updates the group resource and sets the updated Terraform state on success.
func (r *GroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data GroupResourceModel
	var diags diag.Diagnostics

	// Read Terraform plan data into group resource model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Create request body from group data
	updateRequestBody, diags := data.CreateRequestBody()
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	requestData := bytes.NewReader(updateRequestBody)

	// Update group in AAP
	updateResponseBody, diags := r.client.Update(data.URL.ValueString(), requestData)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated group data into group resource model
	diags = data.ParseHttpResponse(updateResponseBody)
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

// Delete deletes the group resource.
func (r *GroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data GroupResourceModel
	var diags diag.Diagnostics

	// Read current Terraform state data into group resource model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete group from AAP
	_, diags = r.client.Delete(data.URL.ValueString())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// CreateRequestBody creates a JSON encoded request body from the group resource data
func (r *GroupResourceModel) CreateRequestBody() ([]byte, diag.Diagnostics) {
	// Convert group resource data to API data model
	group := GroupAPIModel{
		InventoryId: r.InventoryId.ValueInt64(),
		Name:        r.Name.ValueString(),
		Description: r.Description.ValueString(),
		Variables:   r.Variables.ValueString(),
	}

	// Create JSON encoded request body
	jsonBody, err := json.Marshal(group)
	if err != nil {
		var diags diag.Diagnostics
		diags.AddError(
			"Error marshaling request body",
			fmt.Sprintf("Could not create request body for group resource, unexpected error: %s", err.Error()),
		)
		return nil, diags
	}

	return jsonBody, nil
}

// ParseHttpResponse updates the group resource data from an AAP API response
func (r *GroupResourceModel) ParseHttpResponse(body []byte) diag.Diagnostics {
	var diags diag.Diagnostics

	// Unmarshal the JSON response
	var resultApiGroup GroupAPIModel
	err := json.Unmarshal(body, &resultApiGroup)
	if err != nil {
		diags.AddError("Error parsing JSON response from AAP", err.Error())
		return diags
	}

	// Map response to the group resource schema and update attribute values
	r.InventoryId = types.Int64Value(resultApiGroup.InventoryId)
	r.URL = types.StringValue(resultApiGroup.URL)
	r.Id = types.Int64Value(resultApiGroup.Id)
	r.Name = types.StringValue(resultApiGroup.Name)
	r.Description = ParseStringValue(resultApiGroup.Description)
	r.Variables = ParseAAPCustomStringValue(resultApiGroup.Variables)

	return diags
}
