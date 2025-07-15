package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
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

// ConfigValidators returns a list of validators for the data source configuration.
// Overrides BaseDataSource to allow both id and name as lookup options.
func (d *OrganizationDataSource) ConfigValidators(_ context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.Any(
			datasourcevalidator.AtLeastOneOf(
				path.MatchRoot("id"),
				path.MatchRoot("name")),
		),
	}
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
