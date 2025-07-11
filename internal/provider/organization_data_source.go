package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/datasource"
)

// Inventory AAP API model
type OrganizationAPIModel struct {
	BaseDetailAPIModel
}

// InventoryDataSourceModel maps the data source schema data.
type OrganizationDataSourceModel struct {
	BaseDetailSourceModel
}

// InventoryDataSource is the data source implementation.
type OrganizationDataSource struct {
	BaseDataSource
}

// NewInventoryDataSource is a helper function to simplify the provider implementation.
func NewOrganizationDataSource() datasource.DataSource {
	return &OrganizationDataSource{
		BaseDataSource: *NewBaseDataSource(nil, StringDescriptions{
			MetadataEntitySlug:    "organization",
			DescriptiveEntityName: "Organization",
			ApiEntitySlug:         "organizations",
		}),
	}
}
