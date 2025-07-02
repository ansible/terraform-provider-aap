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

type BaseDetailAPIModelDescription struct {
	Description string `json:"description,omitempty"`
}

type BaseDetailAPIModelName struct {
	Name string `json:"name,omitempty"`
}
type BaseDetailAPIModelRelated struct {
	Related RelatedAPIModel `json:"related"`
}

type BaseDetailAPIModelSummaryFields struct {
	SummaryFields SummaryFieldsAPIModel `json:"summary_fields"`
}

type BaseDetailAPIModelVariables struct {
	Variables string `json:"variables,omitempty"`
}

type BaseDetailAPIModelCommon struct {
	BaseDetailAPIModel
	Description string          `json:"description,omitempty"`
	Name        string          `json:"name,omitempty"`
	Related     RelatedAPIModel `json:"related"`
	Variables   string          `json:"variables,omitempty"`
}

type BaseDetailAPIModelWithOrg struct {
	BaseDetailAPIModelCommon
	SummaryFields SummaryFieldsAPIModel `json:"summary_fields"`
	Organization  int64                 `json:"organization"`
}

// ---------------------------------------------------------------------------

type BaseDetailSourceModel struct {
	Id  types.Int64  `tfsdk:"id"`
	URL types.String `tfsdk:"url"`
}

type BaseDetailSourceModelDescription struct {
	Description types.String `tfsdk:"description"`
}

type BaseDetailSourceModelName struct {
	Name types.String `tfsdk:"name"`
}

type BaseDetailSourceModelNamedUrl struct {
	NamedUrl types.String `tfsdk:"named_url"`
}

type BaseDetailSourceModelVariables struct {
	Variables customtypes.AAPCustomStringValue `tfsdk:"variables"`
}

type BaseDetailSourceModelCommon struct {
	BaseDetailSourceModel
	Description types.String                     `tfsdk:"description"`
	Name        types.String                     `tfsdk:"name"`
	NamedUrl    types.String                     `tfsdk:"named_url"`
	Variables   customtypes.AAPCustomStringValue `tfsdk:"variables"`
}

type BaseDetailSourceModelWithOrg struct {
	BaseDetailSourceModelCommon
	Organization     types.Int64  `tfsdk:"organization"`
	OrganizationName types.String `tfsdk:"organization_name"`
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
