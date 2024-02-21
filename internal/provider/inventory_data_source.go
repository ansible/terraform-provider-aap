package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &InventoryDataSource{}
	_ datasource.DataSourceWithConfigure = &InventoryDataSource{}
)

// NewInventoryDataSource is a helper function to simplify the provider implementation.
func NewInventoryDataSource() datasource.DataSource {
	return &InventoryDataSource{}
}

// inventoryDataSource is the data source implementation.
type InventoryDataSource struct {
	client *AAPClient
}

// Metadata returns the data source type name.
func (d *InventoryDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_inventory"
}

// Schema defines the schema for the data source.
func (d *InventoryDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Required:    true,
				Description: "Inventory id",
			},
			"organization": schema.Int64Attribute{
				Computed:    true,
				Description: "Identifier for the organization to which the inventory belongs",
			},
			"url": schema.StringAttribute{
				Computed:    true,
				Description: "Url of the inventory",
			},
			"name": schema.StringAttribute{
				Computed:    true,
				Description: "Name of the inventory",
			},
			"description": schema.StringAttribute{
				Computed:    true,
				Description: "Description of the inventory",
			},
			"variables": schema.StringAttribute{
				Computed:    true,
				Description: "Variables of the inventory",
			},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *InventoryDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state InventoryDataSourceModel
	var diags diag.Diagnostics

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	readResponseBody, diags := d.client.Get("api/v2/inventories/" + state.Id.String())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = state.ParseHttpResponse(readResponseBody)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Set state
	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Configure adds the provider configured client to the data source.
func (d *InventoryDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*AAPClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *AAPClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	d.client = client
}

// inventoryDataSourceModel maps the data source schema data.
type InventoryDataSourceModel struct {
	Id           types.Int64  `tfsdk:"id"`
	Organization types.Int64  `tfsdk:"organization"`
	Url          types.String `tfsdk:"url"`
	Name         types.String `tfsdk:"name"`
	Description  types.String `tfsdk:"description"`
	Variables    types.String `tfsdk:"variables"`
}

func (d *InventoryDataSourceModel) ParseHttpResponse(body []byte) diag.Diagnostics {
	var diags diag.Diagnostics

	// Unmarshal the JSON response
	var apiInventory InventoryAPIModel
	err := json.Unmarshal(body, &apiInventory)
	if err != nil {
		diags.AddError("Error parsing JSON response from AAP", err.Error())
		return diags
	}

	// Map response to the inventory datesource schema
	d.Id = types.Int64Value(apiInventory.Id)
	d.Organization = types.Int64Value(apiInventory.Organization)
	d.Url = types.StringValue(apiInventory.Url)

	d.Name = ParseStringValue(apiInventory.Name)
	d.Description = ParseStringValue(apiInventory.Description)
	d.Variables = ParseStringValue(apiInventory.Variables)

	return diags
}
