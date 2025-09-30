package provider

import (
	"context"
	"fmt"
	"maps"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
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

func (d *EDAEventStreamDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	baseAttrs := d.GetBaseAttributes()
	newAttrs := map[string]schema.Attribute{
		"url": schema.StringAttribute{
			Computed: true,
		},
	}

	maps.Copy(newAttrs, baseAttrs)

	resp.Schema = schema.Schema{
		Attributes:  newAttrs,
		Description: fmt.Sprintf("Creates a %s.", d.DescriptiveEntityName),
	}
}
