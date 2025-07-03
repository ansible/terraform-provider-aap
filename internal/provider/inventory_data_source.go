package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/datasource"
)

// Inventory AAP API model
type InventoryAPIModel struct {
	BaseDetailAPIModelWithOrg
}

// InventoryDataSourceModel maps the data source schema data.
type InventoryDataSourceModel struct {
	BaseDetailSourceModelWithOrg
}

// InventoryDataSource is the data source implementation.
type InventoryDataSource struct {
	BaseDataSourceWithOrg
}

// NewInventoryDataSource is a helper function to simplify the provider implementation.
func NewInventoryDataSource() datasource.DataSource {
	return &InventoryDataSource{
		BaseDataSourceWithOrg: *NewBaseDataSourceWithOrg(nil, StringDescriptions{
			MetadataEntitySlug:    "inventory",
			DescriptiveEntityName: "Inventory",
			ApiEntitySlug:         "inventories",
		}),
	}
}
