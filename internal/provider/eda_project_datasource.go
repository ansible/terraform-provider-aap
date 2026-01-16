package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/datasource"
)

// Ensure the implementation satisfies the desired interfaces.
var _ datasource.DataSource = &EDAProjectDataSource{}

type EDAProjectDataSource struct {
	BaseEdaDataSource
}

func NewEDAProjectDataSource() datasource.DataSource {
	return &EDAProjectDataSource{
		BaseEdaDataSource: *NewBaseEdaDataSource(nil, StringDescriptions{
			MetadataEntitySlug:    "eda_project",
			DescriptiveEntityName: "EDA Project",
			APIEntitySlug:         "projects",
		}),
	}
}
