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
	Id          int64           `json:"id"`
	URL         string          `json:"url"`
	Description string          `json:"description,omitempty"`
	Name        string          `json:"name,omitempty"`
	Related     RelatedAPIModel `json:"related"`
	Variables   string          `json:"variables,omitempty"`
}

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

type BaseDetailSourceModel struct {
	Id          tftypes.Int64                    `tfsdk:"id"`
	URL         tftypes.String                   `tfsdk:"url"`
	Description tftypes.String                   `tfsdk:"description"`
	Name        tftypes.String                   `tfsdk:"name"`
	NamedUrl    tftypes.String                   `tfsdk:"named_url"`
	Variables   customtypes.AAPCustomStringValue `tfsdk:"variables"`
}

// TF representation of the BaseDetailSourceModel with organization information.
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

type BaseEdaDataSource struct {
	client ProviderHTTPClient
	StringDescriptions
}

// BaseResource describes infrastructure objects, such as Jobs, Hosts, or Groups.
// See https://developer.hashicorp.com/terraform/language/resources
type BaseResource struct {
	client ProviderHTTPClient
	StringDescriptions
}

// BaseResourceWithOrg represents a resource with an associated AAP Organization.
type BaseResourceWithOrg struct {
	BaseResource
}

// BaseResourceAPIModel represents the most basic AAP API model for resources.
type BaseResourceAPIModel struct {
	Url string `json:"url"`
}

// BaseResourceModel describes fields in a Terraform resource.
type BaseResourceSourceModel struct {
	Url tftypes.String `tfsdk:"url"`
}
