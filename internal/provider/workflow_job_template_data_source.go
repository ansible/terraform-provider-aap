package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/datasource"
)

// WorkflowJobTemplate AAP API model
type WorkflowJobTemplateAPIModel struct {
	BaseDetailAPIModelWithOrg
}

// WorkflowJobTemplateDataSourceModel maps the data source schema data.
type WorkflowJobTemplateDataSourceModel struct {
	BaseDetailSourceModelWithOrg
}

// WorkflowJobTemplateDataSource is the data source implementation.
type WorkflowJobTemplateDataSource struct {
	BaseDataSourceWithOrg
}

// NewWorkflowJobTemplateDataSource is a helper function to simplify the provider implementation.
func NewWorkflowJobTemplateDataSource() datasource.DataSource {
	return &WorkflowJobTemplateDataSource{
		BaseDataSourceWithOrg: *NewBaseDataSourceWithOrg(nil, StringDescriptions{
			MetadataEntitySlug:    "workflow_job_template",
			DescriptiveEntityName: "WorkflowJobTemplate",
			ApiEntitySlug:         "workflow_job_templates",
		}),
	}
}
