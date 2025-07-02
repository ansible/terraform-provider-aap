package provider

import (
	"github.com/ansible/terraform-provider-aap/internal/provider/customtypes"
	tftypes "github.com/hashicorp/terraform-plugin-framework/types"
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
	Id  tftypes.Int64  `tfsdk:"id"`
	URL tftypes.String `tfsdk:"url"`
}

type BaseDetailSourceModelDescription struct {
	Description tftypes.String `tfsdk:"description"`
}

type BaseDetailSourceModelName struct {
	Name tftypes.String `tfsdk:"name"`
}

type BaseDetailSourceModelNamedUrl struct {
	NamedUrl tftypes.String `tfsdk:"named_url"`
}

type BaseDetailSourceModelVariables struct {
	Variables customtypes.AAPCustomStringValue `tfsdk:"variables"`
}

type BaseDetailSourceModelCommon struct {
	BaseDetailSourceModel
	Description tftypes.String                   `tfsdk:"description"`
	Name        tftypes.String                   `tfsdk:"name"`
	NamedUrl    tftypes.String                   `tfsdk:"named_url"`
	Variables   customtypes.AAPCustomStringValue `tfsdk:"variables"`
}

type BaseDetailSourceModelWithOrg struct {
	BaseDetailSourceModelCommon
	Organization     tftypes.Int64  `tfsdk:"organization"`
	OrganizationName tftypes.String `tfsdk:"organization_name"`
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
