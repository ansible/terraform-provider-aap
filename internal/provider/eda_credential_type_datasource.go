package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"path"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	tftypes "github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &EDACredentialTypeDataSource{}
var _ datasource.DataSourceWithConfigure = &EDACredentialTypeDataSource{}

type EDACredentialTypeDataSource struct {
	client ProviderHTTPClient
}

type EDACredentialTypeDataSourceModel struct {
	ID          tftypes.Int64  `tfsdk:"id"`
	Name        tftypes.String `tfsdk:"name"`
	Description tftypes.String `tfsdk:"description"`
	Inputs      tftypes.String `tfsdk:"inputs"`
	Injectors   tftypes.String `tfsdk:"injectors"`
}

func NewEDACredentialTypeDataSource() datasource.DataSource {
	return &EDACredentialTypeDataSource{}
}

func (d *EDACredentialTypeDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_eda_credential_type"
}

func (d *EDACredentialTypeDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "EDA Credential Type id. Either id or name must be specified.",
			},
			"name": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Name of the EDA Credential Type. Either id or name must be specified.",
			},
			"description": schema.StringAttribute{
				Computed:    true,
				Description: "Description of the EDA Credential Type",
			},
			"inputs": schema.StringAttribute{
				Computed:    true,
				Description: "Input schema for the credential type as a JSON string",
			},
			"injectors": schema.StringAttribute{
				Computed:    true,
				Description: "Injector configuration for the credential type as a JSON string",
			},
		},
		Description: "Gets an existing EDA Credential Type. Either id or name must be specified.",
	}
}

func (d *EDACredentialTypeDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *EDACredentialTypeDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state EDACredentialTypeDataSourceModel
	var diags diag.Diagnostics

	if !DoReadPreconditionsMeet(ctx, resp, d.client) {
		return
	}

	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	hasID := !state.ID.IsNull()
	hasName := !state.Name.IsNull()

	if !hasID && !hasName {
		resp.Diagnostics.AddError(
			"Missing required argument",
			"Either 'id' or 'name' must be specified",
		)
		return
	}

	if hasID && hasName {
		resp.Diagnostics.AddError(
			"Conflicting arguments",
			"Only one of 'id' or 'name' can be specified, not both",
		)
		return
	}

	var credentialTypeID int64

	if hasName {
		edaEndpoint := d.client.getEdaAPIEndpoint()
		if edaEndpoint == "" {
			resp.Diagnostics.AddError(
				"EDA API Endpoint is empty",
				"Expected a valid endpoint but was an empty string. Please report this issue to the provider developers.",
			)
			return
		}
		listURL := path.Join(edaEndpoint, "credential-types")
		params := map[string]string{
			"name": state.Name.ValueString(),
		}
		listResponseBody, diags := d.client.GetWithParams(listURL, params)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		var listResponse BaseEdaAPIModelList
		err := json.Unmarshal(listResponseBody, &listResponse)
		if err != nil {
			resp.Diagnostics.AddError("Error parsing JSON response from AAP", err.Error())
			return
		}

		if len(listResponse.Results) != 1 {
			resp.Diagnostics.AddError(
				"Credential type not found",
				fmt.Sprintf("Expected 1 credential type with name '%s', found %d", state.Name.ValueString(), len(listResponse.Results)),
			)
			return
		}

		credentialTypeID = listResponse.Results[0].Id
	} else {
		credentialTypeID = state.ID.ValueInt64()
	}

	detailURL := fmt.Sprintf("/api/eda/v1/credential-types/%d/", credentialTypeID)
	detailResponseBody, diags := d.client.Get(detailURL)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = state.parseHTTPResponse(detailResponseBody)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (d *EDACredentialTypeDataSourceModel) parseHTTPResponse(body []byte) diag.Diagnostics {
	var diags diag.Diagnostics

	var apiCredentialType EDACredentialTypeAPIModel
	err := json.Unmarshal(body, &apiCredentialType)
	if err != nil {
		diags.AddError("Error parsing JSON response from EDA", err.Error())
		return diags
	}

	d.ID = tftypes.Int64Value(apiCredentialType.ID)
	d.Name = tftypes.StringValue(apiCredentialType.Name)
	d.Description = ParseStringValue(apiCredentialType.Description)

	if len(apiCredentialType.Inputs) > 0 {
		inputsStr := string(apiCredentialType.Inputs)
		if inputsStr != JSONNull && inputsStr != JSONEmptyObject {
			d.Inputs = tftypes.StringValue(inputsStr)
		} else {
			d.Inputs = tftypes.StringNull()
		}
	} else {
		d.Inputs = tftypes.StringNull()
	}

	if len(apiCredentialType.Injectors) > 0 {
		injectorsStr := string(apiCredentialType.Injectors)
		if injectorsStr != JSONNull && injectorsStr != JSONEmptyObject {
			d.Injectors = tftypes.StringValue(injectorsStr)
		} else {
			d.Injectors = tftypes.StringNull()
		}
	} else {
		d.Injectors = tftypes.StringNull()
	}

	return diags
}
