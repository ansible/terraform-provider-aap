package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/datasource"
)

// Organization AAP API model
type OrganizationAPIModel struct {
	BaseDetailAPIModel
}

// OrganizationDataSourceModel maps the data source schema data.
type OrganizationDataSourceModel struct {
	BaseDetailSourceModel
}

// OrganizationDataSource is the data source implementation.
type OrganizationDataSource struct {
	BaseDataSource
}

// NewOrganizationDataSource is a helper function to simplify the provider implementation.
func NewOrganizationDataSource() datasource.DataSource {
	return &OrganizationDataSource{
		BaseDataSource: *NewBaseDataSource(nil, StringDescriptions{
			MetadataEntitySlug:    "organization",
			DescriptiveEntityName: "Organization",
			ApiEntitySlug:         "organizations",
		}),
	}
}
