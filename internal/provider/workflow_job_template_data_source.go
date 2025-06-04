package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/datasource"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource                     = &WorkflowJobTemplateDataSource{}
	_ datasource.DataSourceWithConfigure        = &WorkflowJobTemplateDataSource{}
	_ datasource.DataSourceWithConfigValidators = &WorkflowJobTemplateDataSource{}
	_ datasource.DataSourceWithValidateConfig   = &WorkflowJobTemplateDataSource{}
)

var workflowJobTemplateStringDescriptions = StringDescriptions{
	ApiEntitySlug:         "workflow_job_templates",
	DescriptiveEntityName: "WorkflowJobTemplate",
	MetadataEntitySlug:    "workflow_job_template",
}

// NewWorkflowJobTemplateDataSource is a helper function to simplify the provider implementation.
func NewWorkflowJobTemplateDataSource() datasource.DataSource {
	return &WorkflowJobTemplateDataSource{
		BaseDataSource: *NewBaseDataSource(nil, workflowJobTemplateStringDescriptions),
	}
}

// WorkflowJobTemplateDataSourceModel maps the data source schema data.
type WorkflowJobTemplateDataSourceModel struct {
	BaseDataSourceModel
}

// WorkflowJobTemplate AAP API model
type WorkflowJobTemplateAPIModel struct {
	Id            int64                 `json:"id,omitempty"`
	Organization  int64                 `json:"organization"`
	SummaryFields SummaryFieldsAPIModel `json:"summary_fields,omitempty"`
	Url           string                `json:"url,omitempty"`
	Related       RelatedAPIModel       `json:"related,omitempty"`
	Name          string                `json:"name"`
	Description   string                `json:"description,omitempty"`
	Variables     string                `json:"variables,omitempty"`
}

// WorkflowJobTemplateDataSource is the data source implementation.
type WorkflowJobTemplateDataSource struct {
	BaseDataSource
}
