package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/datasource"
)

// JobTemplateAPIModel represents a JobTemplate AAP API model
type JobTemplateAPIModel struct {
	BaseDetailAPIModelWithOrg
}

// JobTemplateDataSourceModel maps the data source schema data.
type JobTemplateDataSourceModel struct {
	BaseDetailSourceModelWithOrg
}

// JobTemplateDataSource is the data source implementation.
type JobTemplateDataSource struct {
	BaseDataSourceWithOrg
}

// NewJobTemplateDataSource is a helper function to simplify the provider implementation.
func NewJobTemplateDataSource() datasource.DataSource {
	return &JobTemplateDataSource{
		BaseDataSourceWithOrg: *NewBaseDataSourceWithOrg(nil, StringDescriptions{
			MetadataEntitySlug:    "job_template",
			DescriptiveEntityName: "JobTemplate",
			APIEntitySlug:         "job_templates",
		}),
	}
}
