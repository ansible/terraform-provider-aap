package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/datasource"
)

// WorkflowJobTemplateAPIModel represents the AAP API model for workflow job templates.
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
			APIEntitySlug:         "workflow_job_templates",
		}),
	}
}
