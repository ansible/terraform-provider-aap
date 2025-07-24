package provider

import (
	"context"

	"path"

	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	tfpath "github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Organization AAP API model
type OrganizationAPIModel struct {
	BaseDetailAPIModel
}

// OrganizationDataSourceModel maps the data source schema data.
type OrganizationDataSourceModel struct {
	BaseDetailSourceModel
}

// OrganizationDataSource is the data source implementation.
type OrganizationDataSource struct {
	BaseDataSource
}

// ConfigValidators returns a list of validators for the data source configuration.
// Overrides BaseDataSource to allow both id and name as lookup options.
func (d *OrganizationDataSource) ConfigValidators(_ context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.Any(
			datasourcevalidator.AtLeastOneOf(
				tfpath.MatchRoot("id"),
				tfpath.MatchRoot("name")),
		),
	}
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

// ---------------------------------------------------------------------------
// ValidateConfig
// ---------------------------------------------------------------------------

func (d *OrganizationDataSource) ValidateConfig(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	// Check that the response and diagnostics pointer is defined
	if resp == nil {
		tflog.Error(ctx, "Response not defined, we cannot continue with the execution")
		return
	}

	// Check that the current context is active
	if !IsContextActive("ValidateConfig", ctx, &resp.Diagnostics) {
		return
	}

	var data BaseDetailSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if IsValueProvidedOrPromised(data.Id) {
		return
	}

	if IsValueProvidedOrPromised(data.Name) {
		return
	}

	if !IsValueProvidedOrPromised(data.Id) {
		resp.Diagnostics.AddAttributeWarning(
			tfpath.Root("id"),
			"Missing Attribute Configuration",
			"Expected [id] or [Name]",
		)
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *OrganizationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state OrganizationDataSourceModel
	var diags diag.Diagnostics

	// Check Read preconditions
	if !DoReadPreconditionsMeet(ctx, resp, d.client) {
		return
	}

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	uri := path.Join(d.client.getApiEndpoint(), d.ApiEntitySlug)
	resourceURL, err := state.CreateNamedURL(uri, &OrganizationAPIModel{
		BaseDetailAPIModel: BaseDetailAPIModel{
			Id:   state.Id.ValueInt64(),
			Name: state.Name.ValueString(),
		},
	})
	if err != nil {
		resp.Diagnostics.AddError("Minimal Data Not Supplied", "Expected [id] or [Name]")
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
