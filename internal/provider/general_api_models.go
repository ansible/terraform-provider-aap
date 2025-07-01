package provider

import (
	"github.com/ansible/terraform-provider-aap/internal/provider/customtypes"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ---------------------------------------------------------------------------

type RelatedAPIModel struct {
	NamedUrl string `json:"named_url,omitempty"`
}

type SummaryField struct {
	Id   int64  `json:"id"`
	Name string `json:"name"`
}

type SummaryFieldsAPIModel struct {
	Organization SummaryField `json:"organization,omitempty"`
	Inventory    SummaryField `json:"inventory,omitempty"`
}

type BaseDetailAPIModel struct {
	Id  int64  `json:"id"`
	URL string `json:"url"`
}

type BaseDetailAPIModelWithOrg struct {
	BaseDetailAPIModel
	Description   string                `json:"description,omitempty"`
	Name          string                `json:"name,omitempty"`
	Organization  int64                 `json:"organization"`
	Related       RelatedAPIModel       `json:"related"`
	SummaryFields SummaryFieldsAPIModel `json:"summary_fields"`
	Variables     string                `json:"variables,omitempty"`
}

// ---------------------------------------------------------------------------

type BaseDetailDataSourceModel struct {
	Id  types.Int64  `tfsdk:"id"`
	URL types.String `tfsdk:"url"`
}

type BaseDetailDataSourceModelWithNamedUrl struct {
	BaseDetailDataSourceModel
	Name        types.String                     `tfsdk:"name"`
	Description types.String                     `tfsdk:"description"`
	NamedUrl    types.String                     `tfsdk:"named_url"`
	Variables   customtypes.AAPCustomStringValue `tfsdk:"variables"`
}

type BaseDetailDataSourceModelWithOrg struct {
	BaseDetailDataSourceModel
	Name             types.String                     `tfsdk:"name"`
	Description      types.String                     `tfsdk:"description"`
	NamedUrl         types.String                     `tfsdk:"named_url"`
	Variables        customtypes.AAPCustomStringValue `tfsdk:"variables"`
	Organization     types.Int64                      `tfsdk:"organization"`
	OrganizationName types.String                     `tfsdk:"organization_name"`
}

type StringDescriptions struct {
	ApiEntitySlug         string
	DescriptiveEntityName string
	MetadataEntitySlug    string
}

// A struct to represent a base DataSource object, with a client and the slug name of
// the API entity.
type BaseDataSource struct {
	client ProviderHTTPClient
	StringDescriptions
}

type BaseDataSourceWithOrg struct {
	BaseDataSource
}

type BaseDataSourceWithNamedUrl struct {
	BaseDataSource
}
