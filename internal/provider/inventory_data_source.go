package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
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
				Required: true,
			},
			"organization": schema.Int64Attribute{
				Computed: true,
				Description: "Identifier for the organization the inventory should be created in. " +
					"If not provided, the inventory will be created in the default organization.",
			},
			"url": schema.StringAttribute{
				Computed: true,
			},
			"name": schema.StringAttribute{
				Computed: true,
			},
			"description": schema.StringAttribute{
				Computed: true,
			},
			"variables": schema.StringAttribute{
				Optional:   true,
				CustomType: jsontypes.NormalizedType{},
			},
		},
	}
}

// inventoryDataSourceModel maps the data source schema data.
type InventoryDataSourceModel struct {
	Id           types.Int64          `tfsdk:"id"`
	Organization types.Int64          `tfsdk:"organization"`
	Url          types.String         `tfsdk:"url"`
	Name         types.String         `tfsdk:"name"`
	Description  types.String         `tfsdk:"description"`
	Variables    jsontypes.Normalized `tfsdk:"variables"`
}

func (d *InventoryDataSource) ReadInventory(id string) (*InventoryDataSourceModel, error) {
	resp, body, err := d.client.doRequest("GET", "api/v2/inventories/"+id+"/", nil)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, fmt.Errorf("the server response is null")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status: %d, body: %s", resp.StatusCode, body)
	}
	var apiInventory InventoryAPIModel

	// return nil, fmt.Errorf("body %s", body)
	err = json.Unmarshal(body, &apiInventory)
	if err != nil {
		return nil, err
	}

	// Create InventoryDataSourceModel
	inventory := &InventoryDataSourceModel{
		Id:           types.Int64Value(apiInventory.Id),
		Organization: types.Int64Value(apiInventory.Organization),
		Url:          types.StringValue(apiInventory.Url),
		Name:         types.StringValue(apiInventory.Name),
		Description:  types.StringValue(apiInventory.Description),
		Variables:    jsontypes.NewNormalizedValue(apiInventory.Variables),
	}

	return inventory, nil

}

// Read refreshes the Terraform state with the latest data.
func (d *InventoryDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state InventoryDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)

	result, err := d.ReadInventory(state.Id.String())
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read Inventory",
			fmt.Sprintf("%s %s", result, err.Error()),
		)
		return
	}

	// Set state
	diags := resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
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
