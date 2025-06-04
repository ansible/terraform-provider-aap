package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/datasource"
)

// JobTemplate AAP API model
type JobTemplateAPIModel struct {
	Id            int64                 `json:"id,omitempty"`
	Organization  int64                 `json:"organization"`
	SummaryFields SummaryFieldsAPIModel `json:"summary_fields,omitempty"`
	Url           string                `json:"url,omitempty"`
	Related       RelatedAPIModel       `json:"related,omitempty"`
	Name          string                `json:"name"`
	Description   string                `json:"description,omitempty"`
	Variables     string                `json:"variables,omitempty"`
}

// JobTemplateDataSourceModel maps the data source schema data.
type JobTemplateDataSourceModel struct {
	BaseDataSourceModel
}

// JobTemplateDataSource is the data source implementation.
type JobTemplateDataSource struct {
	BaseDataSource
}

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource                     = &JobTemplateDataSource{}
	_ datasource.DataSourceWithConfigure        = &JobTemplateDataSource{}
	_ datasource.DataSourceWithConfigValidators = &JobTemplateDataSource{}
	_ datasource.DataSourceWithValidateConfig   = &JobTemplateDataSource{}
)

var jobTemplatesStringDescriptions = StringDescriptions{
	ApiEntitySlug:         "job_templates",
	DescriptiveEntityName: "JobTemplate",
	MetadataEntitySlug:    "inventory",
}

// NewJobTemplateDataSource is a helper function to simplify the provider implementation.
func NewJobTemplateDataSource() datasource.DataSource {
	return &JobTemplateDataSource{
		BaseDataSource: *NewBaseDataSource(nil, jobTemplatesStringDescriptions),
	}
}
