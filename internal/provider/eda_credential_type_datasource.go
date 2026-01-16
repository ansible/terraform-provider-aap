package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/datasource"
)

// Ensure the implementation satisfies the desired interfaces.
var _ datasource.DataSource = &EDACredentialTypeDataSource{}

type EDACredentialTypeDataSource struct {
	BaseEdaDataSource
}

func NewEDACredentialTypeDataSource() datasource.DataSource {
	return &EDACredentialTypeDataSource{
		BaseEdaDataSource: *NewBaseEdaDataSource(nil, StringDescriptions{
			MetadataEntitySlug:    "eda_credential_type",
			DescriptiveEntityName: "EDA Credential Type",
			APIEntitySlug:         "credential-types",
		}),
	}
}
