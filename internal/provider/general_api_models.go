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
	Id            int64                 `json:"id"`
	Name          string                `json:"name,omitempty"`
	Description   string                `json:"description,omitempty"`
	URL           string                `json:"url"`
	Related       RelatedAPIModel       `json:"related"`
	SummaryFields SummaryFieldsAPIModel `json:"summary_fields"`
	Variables     string                `json:"variables,omitempty"`
}

type BaseDetailAPIModelWithOrg struct {
	BaseDetailAPIModel
	Organization int64 `json:"organization"`
}

// ---------------------------------------------------------------------------

type BaseDetailDataSourceModel struct {
	Id          tftypes.Int64                    `tfsdk:"id"`
	Name        tftypes.String                   `tfsdk:"name"`
	Description tftypes.String                   `tfsdk:"description"`
	URL         tftypes.String                   `tfsdk:"url"`
	NamedUrl    tftypes.String                   `tfsdk:"named_url"`
	Variables   customtypes.AAPCustomStringValue `tfsdk:"variables"`
}

type BaseDetailDataSourceModelWithOrg struct {
	BaseDetailDataSourceModel
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
