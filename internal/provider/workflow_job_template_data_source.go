package provider

import (
	"context"

	"github.com/ansible/terraform-provider-aap/internal/provider/customtypes"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource                     = &WorkflowJobTemplateDataSource{}
	_ datasource.DataSourceWithConfigure        = &WorkflowJobTemplateDataSource{}
	_ datasource.DataSourceWithConfigValidators = &WorkflowJobTemplateDataSource{}
	_ datasource.DataSourceWithValidateConfig   = &WorkflowJobTemplateDataSource{}
)

// NewWorkflowJobTemplateDataSource is a helper function to simplify the provider implementation.
func NewWorkflowJobTemplateDataSource() datasource.DataSource {
	return &WorkflowJobTemplateDataSource{
		BaseDataSource: *NewBaseDataSource(nil, "workflow_job_templates"),
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

// Metadata returns the data source type name.
func (d *WorkflowJobTemplateDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workflow_job_template"
}

// Schema defines the schema for the data source.
func (d *WorkflowJobTemplateDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Optional:    true,
				Description: "WorkflowJobTemplate id",
			},
			"organization": schema.Int64Attribute{
				Computed:    true,
				Description: "Identifier for the organization to which the WorkflowJobTemplate belongs",
			},
			"organization_name": schema.StringAttribute{
				Computed:    true,
				Optional:    true,
				Description: "The name for the organization to which the WorkflowJobTemplate belongs",
			},
			"url": schema.StringAttribute{
				Computed:    true,
				Description: "Url of the WorkflowJobTemplate",
			},
			"named_url": schema.StringAttribute{
				Computed:    true,
				Description: "The Named Url of the WorkflowJobTemplate",
			},
			"name": schema.StringAttribute{
				Computed:    true,
				Optional:    true,
				Description: "Name of the WorkflowJobTemplate",
			},
			"description": schema.StringAttribute{
				Computed:    true,
				Description: "Description of the WorkflowJobTemplate",
			},
			"variables": schema.StringAttribute{
				Computed:    true,
				CustomType:  customtypes.AAPCustomStringType{},
				Description: "Variables of the WorkflowJobTemplate. Will be either JSON or YAML string depending on how the variables were entered into AAP.",
			},
		},
		Description: `Get an existing WorkflowJobTemplate.`,
	}
}
