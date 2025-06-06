package provider

import (
	"context"

	"github.com/ansible/terraform-provider-aap/internal/provider/customtypes"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

// inventoryDataSourceModel maps the data source schema data.
type InventoryDataSourceModel struct {
	BaseDataSourceModel
}

// InventoryDataSource is the data source implementation.
type InventoryDataSource struct {
	BaseDataSource
}

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource                     = &InventoryDataSource{}
	_ datasource.DataSourceWithConfigure        = &InventoryDataSource{}
	_ datasource.DataSourceWithConfigValidators = &InventoryDataSource{}
	_ datasource.DataSourceWithValidateConfig   = &InventoryDataSource{}
)

// NewInventoryDataSource is a helper function to simplify the provider implementation.
func NewInventoryDataSource() datasource.DataSource {
	return &InventoryDataSource{
		BaseDataSource: *NewBaseDataSource(nil, "inventories"),
	}
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
				Optional:    true,
				Description: "Inventory id",
			},
			"organization": schema.Int64Attribute{
				Computed:    true,
				Description: "Identifier for the organization to which the inventory belongs",
			},
			"organization_name": schema.StringAttribute{
				Computed:    true,
				Optional:    true,
				Description: "The name for the organization to which the inventory belongs",
			},
			"url": schema.StringAttribute{
				Computed:    true,
				Description: "Url of the inventory",
			},
			"named_url": schema.StringAttribute{
				Computed:    true,
				Description: "The Named Url of the inventory",
			},
			"name": schema.StringAttribute{
				Computed:    true,
				Optional:    true,
				Description: "Name of the inventory",
			},
			"description": schema.StringAttribute{
				Computed:    true,
				Description: "Description of the inventory",
			},
			"variables": schema.StringAttribute{
				Computed:    true,
				CustomType:  customtypes.AAPCustomStringType{},
				Description: "Variables of the inventory. Will be either JSON or YAML string depending on how the variables were entered into AAP.",
			},
		},
		Description: `Get an existing inventory.`,
	}
}
