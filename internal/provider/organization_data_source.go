package provider

import (
	"context"
	"fmt"
	"path"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
)

// Organization AAP API model
type OrganizationAPIModel struct {
	BaseDetailAPIModel
}

// organizationDataSourceModel maps the data source schema data.
type OrganizationDataSourceModel struct {
	BaseDetailDataSourceModel
}

// OrganizationDataSource is the data source implementation.
type OrganizationDataSource struct {
	BaseDataSource
}

// NewOrganizationDataSource is a helper function to simplify the provider implementation.
func NewOrganizationDataSource() datasource.DataSource {
	return &OrganizationDataSource{
		BaseDataSource: *NewBaseDataSource(nil, StringDescriptions{
			MetadataEntitySlug:    "organization",
			DescriptiveEntityName: "Organization",
			ApiEntitySlug:         "organizations",
		}),
	}
}

// Schema defines the schema for the data source.
func (d *OrganizationDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = GetBaseSchema(d.DescriptiveEntityName, d.MetadataEntitySlug)
}

// Read refreshes the Terraform state with the latest data.
func (d *OrganizationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state OrganizationDataSourceModel
	var diags diag.Diagnostics

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	uri := path.Join(d.client.getApiEndpoint(), "organizations")
	resourceURL, err := ReturnAAPNamedURLWithoutOrganization(state.Id, state.Name, uri)

	if err != nil {
		resp.Diagnostics.AddError("Minimal Data Not Supplied", "Expected either [id] or [name]")
		return
	}

	readResponseBody, diags := d.client.Get(resourceURL)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	diags = state.ParseHttpResponse(readResponseBody)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Set state
	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Configure adds the provider configured client to the data source.
func (d *OrganizationDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*AAPClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *AAPClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	d.client = client
}

func (d *OrganizationDataSource) ConfigValidators(_ context.Context) []datasource.ConfigValidator {
	// You have at least an id or a name + organization_name pair
	return GetIdOrNameDataSourceConfigValidator()
}

func (d *OrganizationDataSource) ValidateConfig(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	var data OrganizationDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	AppendIdOrNameConfigurationValidationResults(resp, data.Id, data.Name)
}
