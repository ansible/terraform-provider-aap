package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/datasource"
)

// Ensure the implementation satisfies the desired interfaces.
var _ datasource.DataSource = &EDAEventStreamDataSource{}

type EDAEventStreamDataSource struct {
	BaseEdaDataSource
}

func NewEDAEventStreamDataSource() datasource.DataSource {
	return &EDAEventStreamDataSource{
		BaseEdaDataSource: *NewBaseEdaDataSource(nil, StringDescriptions{
			MetadataEntitySlug:    "eda_eventstream",
			DescriptiveEntityName: "EDA Event Stream",
			ApiEntitySlug:         "event-streams",
		}),
	}
}
