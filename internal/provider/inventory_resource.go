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

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource              = &InventoryResource{}
	_ resource.ResourceWithConfigure = &InventoryResource{}
)

// NewInventoryResource is a helper function to simplify the provider implementation.
func NewInventoryResource() resource.Resource {
	return &InventoryResource{}
}

// InventoryResource is the resource implementation.
type InventoryResource struct {
	client ProviderHTTPClient
}

// Metadata returns the resource type name.
func (r *InventoryResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_inventory"
}

// Configure adds the provider configured client to the resource.
func (r *InventoryResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// Schema defines the schema for the resource.
func (r *InventoryResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
				Description: "Inventory id",
			},
			"organization": schema.Int64Attribute{
				Computed: true,
				Optional: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
				Description: "Identifier for the organization the inventory should be created in. " +
					"If not provided, the inventory will be created in the default organization.",
			},
			"organization_name": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Description: "Name for the organization.",
			},
			"url": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Description: "URL of the inventory",
			},
			"named_url": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Description: "Named URL of the inventory",
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the inventory",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Description for the inventory",
			},
			"variables": schema.StringAttribute{
				Description: "Inventory variables. Must be provided as either a JSON or YAML string.",
				Optional:    true,
				CustomType:  customtypes.AAPCustomStringType{},
			},
		},
		Description: `Creates an inventory.`,
	}
}

// Create creates the inventory resource and sets the Terraform state on success.
func (r *InventoryResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data inventoryResourceModel
	var diags diag.Diagnostics

	// Read Terraform plan data into inventory resource model
	diags = req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Generate request body from inventory data
	createRequestBody, diags := data.generateRequestBody()
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	requestData := bytes.NewReader(createRequestBody)

	// Create new inventory in AAP
	inventoriesURL := path.Join(r.client.getApiEndpoint(), "inventories")
	createResponseBody, diags := r.client.Create(inventoriesURL, requestData)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save new inventory data into inventory resource model
	diags = data.parseHTTPResponse(createResponseBody)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated state
	diags = resp.State.Set(ctx, data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Read refreshes the Terraform state with the latest inventory data.
func (r *InventoryResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data inventoryResourceModel
	var diags diag.Diagnostics

	// Read current Terraform state data into inventory resource model
	diags = req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get latest inventory data from AAP
	readResponseBody, diags := r.client.Get(data.Url.ValueString())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save latest inventory data into inventory resource model
	diags = data.parseHTTPResponse(readResponseBody)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated state
	diags = resp.State.Set(ctx, data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update updates the inventory resource and sets the updated Terraform state on success.
func (r *InventoryResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data inventoryResourceModel
	var diags diag.Diagnostics

	// Read Terraform plan data into inventory resource model
	diags = req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Generate request body from inventory data
	updateRequestBody, diags := data.generateRequestBody()
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	requestData := bytes.NewReader(updateRequestBody)

	// Update inventory in AAP
	updateResponseBody, diags := r.client.Update(data.Url.ValueString(), requestData)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated inventory data into inventory resource model
	diags = data.parseHTTPResponse(updateResponseBody)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated state
	diags = resp.State.Set(ctx, data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete deletes the inventory resource.
func (r *InventoryResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data inventoryResourceModel
	var diags diag.Diagnostics

	// Read current Terraform state data into inventory resource model
	diags = req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete inventory from AAP
	_, diags = r.client.Delete(data.Url.ValueString())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// InventoryResourceModel maps the inventory resource schema to a Go struct.
type inventoryResourceModel struct {
	Id           types.Int64                      `tfsdk:"id"`
	Organization types.Int64                      `tfsdk:"organization"`
	OrganizationName types.String                 `tfsdk:"organization_name"`
	Url          types.String                     `tfsdk:"url"`
	NamedUrl     types.String                     `tfsdk:"named_url"`
	Name         types.String                     `tfsdk:"name"`
	Description  types.String                     `tfsdk:"description"`
	Variables    customtypes.AAPCustomStringValue `tfsdk:"variables"`
}

// generateRequestBody creates a JSON encoded request body from the inventory resource data.
func (r *inventoryResourceModel) generateRequestBody() ([]byte, diag.Diagnostics) {
	// Convert inventory resource data to API data model
	var organizationId int64

	// Use default organization if not provided
	if r.Organization.ValueInt64() == 0 {
		organizationId = 1
	} else {
		organizationId = r.Organization.ValueInt64()
	}
	inventory := InventoryAPIModel{
		Organization: organizationId,
		Name:         r.Name.ValueString(),
		Description:  r.Description.ValueString(),
		Variables:    r.Variables.ValueString(),
	}

	// Generate JSON encoded request body
	jsonBody, err := json.Marshal(inventory)
	if err != nil {
		var diags diag.Diagnostics
		diags.AddError(
			"Error marshaling request body",
			fmt.Sprintf("Could not generate request body for inventory resource, unexpected error: %s", err.Error()),
		)
		return nil, diags
	}

	return jsonBody, nil
}

// parseHTTPResponse updates the inventory resource data from an AAP API response.
func (r *inventoryResourceModel) parseHTTPResponse(body []byte) diag.Diagnostics {
	var parseResponseDiags diag.Diagnostics

	// Unmarshal the JSON response
	var apiInventory InventoryAPIModel
	err := json.Unmarshal(body, &apiInventory)
	if err != nil {
		parseResponseDiags.AddError("Error parsing JSON response from AAP", err.Error())
		return parseResponseDiags
	}

	// Map response to the inventory resource schema and update attribute values
	r.Id = types.Int64Value(apiInventory.Id)
	r.Organization = types.Int64Value(apiInventory.Organization)
	r.OrganizationName = types.StringValue(apiInventory.SummaryFields.Organization.Name)
	r.Url = types.StringValue(apiInventory.Url)
	r.NamedUrl = types.StringValue(apiInventory.Related.NamedUrl)
	r.Name = types.StringValue(apiInventory.Name)
	r.Description = ParseStringValue(apiInventory.Description)
	r.Variables = ParseAAPCustomStringValue(apiInventory.Variables)

	return parseResponseDiags
}

// Inventory AAP API model
type InventoryAPIModel struct {
	Id           int64  `json:"id,omitempty"`
	Organization int64  `json:"organization"`
	SummaryFields SummaryFieldsAPIModel `json:"summary_fields,omitempty"`
	Url          string `json:"url,omitempty"`
	Related          RelatedAPIModel `json:"related,omitempty"`
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	Variables    string `json:"variables,omitempty"`
}

type SummaryFieldsAPIModel struct{
	Organization OrganizationAPIModel `json:"organization"`
}

type OrganizationAPIModel struct{
	Id           int64  `json:"id,omitempty"`
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
}

type RelatedAPIModel struct{
	NamedUrl     string `json:"named_url,omitempty"`
}