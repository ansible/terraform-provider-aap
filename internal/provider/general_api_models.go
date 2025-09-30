package provider

import (
	"github.com/ansible/terraform-provider-aap/internal/provider/customtypes"
	tftypes "github.com/hashicorp/terraform-plugin-framework/types"
)

// ---------------------------------------------------------------------------

// RelatedAPIModel represents related API model data
type RelatedAPIModel struct {
	NamedURL string `json:"named_url,omitempty"`
}

// SummaryField represents a summary field in AAP API responses.
type SummaryField struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// SummaryFieldsAPIModel represents the summary_fields section in AAP API responses.
type SummaryFieldsAPIModel struct {
	Organization SummaryField `json:"organization,omitempty"`
	Inventory    SummaryField `json:"inventory,omitempty"`
}

// BaseDetailAPIModel represents the base structure for AAP API detail responses.
type BaseDetailAPIModel struct {
	ID          int64           `json:"id"`
	URL         string          `json:"url"`
	Description string          `json:"description,omitempty"`
	Name        string          `json:"name,omitempty"`
	Related     RelatedAPIModel `json:"related"`
	Variables   string          `json:"variables,omitempty"`
}

// BaseDetailAPIModelWithOrg represents the base structure for AAP API detail responses with organization information.
type BaseDetailAPIModelWithOrg struct {
	BaseDetailAPIModel
	SummaryFields SummaryFieldsAPIModel `json:"summary_fields"`
	Organization  int64                 `json:"organization"`
}

type BaseEdaAPIModel struct {
	Name string `json:"name"`
	Id   int64  `json:"id"`
	URL  string `json:"url"`
}

type BaseEdaAPIModelList struct {
	Results []BaseEdaAPIModel `json:"results"`
}

// ---------------------------------------------------------------------------

// BaseDetailSourceModel represents the Terraform data source model for base detail resources.
type BaseDetailSourceModel struct {
	ID          tftypes.Int64                    `tfsdk:"id"`
	URL         tftypes.String                   `tfsdk:"url"`
	Description tftypes.String                   `tfsdk:"description"`
	Name        tftypes.String                   `tfsdk:"name"`
	NamedURL    tftypes.String                   `tfsdk:"named_url"`
	Variables   customtypes.AAPCustomStringValue `tfsdk:"variables"`
}

// BaseDetailSourceModelWithOrg represents the Terraform data source model with organization information.
type BaseDetailSourceModelWithOrg struct {
	BaseDetailSourceModel
	Organization     tftypes.Int64  `tfsdk:"organization"`
	OrganizationName tftypes.String `tfsdk:"organization_name"`
}

type BaseEdaSourceModel struct {
	ID   tftypes.Int64  `tfsdk:"id"`
	Name tftypes.String `tfsdk:"name"`
	URL  tftypes.String `tfsdk:"url"`
}

type StringDescriptions struct {
	APIEntitySlug         string
	DescriptiveEntityName string
	MetadataEntitySlug    string
}

// BaseDataSource represents a base DataSource object with a client and the slug name of
// the API entity.
type BaseDataSource struct {
	client HTTPClient
	StringDescriptions
}

// BaseDataSourceWithOrg represents a base DataSource object with organization support.
type BaseDataSourceWithOrg struct {
	BaseDataSource
}

type BaseEdaDataSource struct {
	client ProviderHTTPClient
	StringDescriptions
}

// BaseResource describes infrastructure objects, such as Jobs, Hosts, or Groups.
// See https://developer.hashicorp.com/terraform/language/resources
type BaseResource struct {
	client HTTPClient
	StringDescriptions
}

// BaseResourceWithOrg represents a resource with an associated AAP Organization.
type BaseResourceWithOrg struct {
	BaseResource
}

// BaseResourceAPIModel represents the most basic AAP API model for resources.
type BaseResourceAPIModel struct {
	URL string `json:"url"`
}

// BaseResourceSourceModel describes fields in a Terraform resource.
type BaseResourceSourceModel struct {
	URL tftypes.String `tfsdk:"url"`
}
