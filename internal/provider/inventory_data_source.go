package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/datasource"
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

var inventoriesStringDescriptions = StringDescriptions{
	ApiEntitySlug:         "inventories",
	DescriptiveEntityName: "Inventory",
	MetadataEntitySlug:    "inventory",
}

// NewInventoryDataSource is a helper function to simplify the provider implementation.
func NewInventoryDataSource() datasource.DataSource {
	return &InventoryDataSource{
		BaseDataSource: *NewBaseDataSource(nil, inventoriesStringDescriptions),
	}
}
