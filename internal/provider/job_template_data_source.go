package provider

import (
	"context"

	"github.com/ansible/terraform-provider-aap/internal/provider/customtypes"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
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

// NewJobTemplateDataSource is a helper function to simplify the provider implementation.
func NewJobTemplateDataSource() datasource.DataSource {
	return &JobTemplateDataSource{
		BaseDataSource: *NewBaseDataSource(nil, "job_templates"),
	}
}

// Metadata returns the data source type name.
func (d *JobTemplateDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_job_template"
}

// Schema defines the schema for the data source.
func (d *JobTemplateDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Optional:    true,
				Description: "JobTemplate id",
			},
			"organization": schema.Int64Attribute{
				Computed:    true,
				Description: "Identifier for the organization to which the JobTemplate belongs",
			},
			"organization_name": schema.StringAttribute{
				Computed:    true,
				Optional:    true,
				Description: "The name for the organization to which the JobTemplate belongs",
			},
			"url": schema.StringAttribute{
				Computed:    true,
				Description: "Url of the JobTemplate",
			},
			"named_url": schema.StringAttribute{
				Computed:    true,
				Description: "The Named Url of the JobTemplate",
			},
			"name": schema.StringAttribute{
				Computed:    true,
				Optional:    true,
				Description: "Name of the JobTemplate",
			},
			"description": schema.StringAttribute{
				Computed:    true,
				Description: "Description of the JobTemplate",
			},
			"variables": schema.StringAttribute{
				Computed:    true,
				CustomType:  customtypes.AAPCustomStringType{},
				Description: "Variables of the JobTemplate. Will be either JSON or YAML string depending on how the variables were entered into AAP.",
			},
		},
		Description: `Get an existing JobTemplate.`,
	}
}
