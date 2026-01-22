package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	tftypes "github.com/hashicorp/terraform-plugin-framework/types"
)

type EDACredentialTypeResourceModel struct {
	ID          tftypes.Int64  `tfsdk:"id"`
	Name        tftypes.String `tfsdk:"name"`
	Description tftypes.String `tfsdk:"description"`
	Inputs      tftypes.String `tfsdk:"inputs"`
	Injectors   tftypes.String `tfsdk:"injectors"`
}

type EDACredentialTypeAPIModel struct {
	ID          int64           `json:"id,omitempty"`
	URL         string          `json:"url,omitempty"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Inputs      json.RawMessage `json:"inputs,omitempty"`
	Injectors   json.RawMessage `json:"injectors,omitempty"`
}

type EDACredentialTypeResource struct {
	client ProviderHTTPClient
}

var (
	_ resource.Resource              = &EDACredentialTypeResource{}
	_ resource.ResourceWithConfigure = &EDACredentialTypeResource{}
)

func NewEDACredentialTypeResource() resource.Resource {
	return &EDACredentialTypeResource{}
}

func (r *EDACredentialTypeResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_eda_credential_type"
}

func (r *EDACredentialTypeResource) Configure(_ context.Context, req resource.ConfigureRequest,
	resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*AAPClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *AAPClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func (r *EDACredentialTypeResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
				Description: "EDA Credential Type id",
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the EDA Credential Type",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Description for the EDA Credential Type",
			},
			"inputs": schema.StringAttribute{
				Optional:    true,
				Description: "Input schema for the credential type. Must be provided as a JSON string.",
			},
			"injectors": schema.StringAttribute{
				Optional:    true,
				Description: "Injector configuration for the credential type. Must be provided as a JSON string.",
			},
		},
		Description: `Creates an EDA credential type.`,
	}
}

func (r *EDACredentialTypeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data EDACredentialTypeResourceModel
	var diags diag.Diagnostics

	diags = req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createRequestBody, diags := data.generateRequestBody()
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	requestData := bytes.NewReader(createRequestBody)

	edaEndpoint := r.client.getEdaAPIEndpoint()
	if edaEndpoint == "" {
		resp.Diagnostics.AddError(
			"EDA API Endpoint is empty",
			"Expected a valid endpoint but was an empty string. Please report this issue to the provider developers.",
		)
		return
	}
	credentialTypesURL := path.Join(edaEndpoint, "credential-types")
	createResponseBody, diags := r.client.Create(credentialTypesURL, requestData)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = data.parseHTTPResponse(createResponseBody)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *EDACredentialTypeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data EDACredentialTypeResourceModel
	var diags diag.Diagnostics

	diags = req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("/api/eda/v1/credential-types/%d/", data.ID.ValueInt64())
	readResponseBody, diags := r.client.Get(url)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = data.parseHTTPResponse(readResponseBody)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *EDACredentialTypeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data EDACredentialTypeResourceModel
	var diags diag.Diagnostics

	diags = req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateRequestBody, diags := data.generateRequestBody()
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	requestData := bytes.NewReader(updateRequestBody)

	url := fmt.Sprintf("/api/eda/v1/credential-types/%d/", data.ID.ValueInt64())
	updateResponseBody, diags := r.client.Patch(url, requestData)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = data.parseHTTPResponse(updateResponseBody)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *EDACredentialTypeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data EDACredentialTypeResourceModel
	var diags diag.Diagnostics

	diags = req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("/api/eda/v1/credential-types/%d/", data.ID.ValueInt64())
	_, diags = r.client.Delete(url)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *EDACredentialTypeResourceModel) generateRequestBody() ([]byte, diag.Diagnostics) {
	var diags diag.Diagnostics

	credentialType := EDACredentialTypeAPIModel{
		Name:        r.Name.ValueString(),
		Description: r.Description.ValueString(),
	}

	if !r.Inputs.IsNull() && r.Inputs.ValueString() != "" {
		credentialType.Inputs = json.RawMessage(strings.TrimSpace(r.Inputs.ValueString()))
	}

	if !r.Injectors.IsNull() && r.Injectors.ValueString() != "" {
		credentialType.Injectors = json.RawMessage(strings.TrimSpace(r.Injectors.ValueString()))
	}

	jsonBody, err := json.Marshal(credentialType)
	if err != nil {
		diags.AddError(
			"Error marshaling request body",
			fmt.Sprintf("Could not generate request body for EDA credential type resource, unexpected error: %s", err.Error()),
		)
		return nil, diags
	}

	return jsonBody, nil
}

func (r *EDACredentialTypeResourceModel) parseHTTPResponse(body []byte) diag.Diagnostics {
	var parseResponseDiags diag.Diagnostics

	var apiCredentialType EDACredentialTypeAPIModel
	err := json.Unmarshal(body, &apiCredentialType)
	if err != nil {
		parseResponseDiags.AddError("Error parsing JSON response from EDA", err.Error())
		return parseResponseDiags
	}

	r.ID = tftypes.Int64Value(apiCredentialType.ID)
	r.Name = tftypes.StringValue(apiCredentialType.Name)
	r.Description = ParseStringValue(apiCredentialType.Description)

	inputsStr := strings.TrimSpace(string(apiCredentialType.Inputs))
	if len(inputsStr) > 0 && inputsStr != JSONEmptyObject && inputsStr != JSONNull {
		var inputsObj interface{}
		if err := json.Unmarshal([]byte(inputsStr), &inputsObj); err == nil {
			if normalized, err := json.Marshal(inputsObj); err == nil {
				inputsStr = string(normalized)
			}
		}
		r.Inputs = tftypes.StringValue(inputsStr)
	} else {
		r.Inputs = tftypes.StringNull()
	}

	injectorsStr := strings.TrimSpace(string(apiCredentialType.Injectors))
	if len(injectorsStr) > 0 && injectorsStr != JSONEmptyObject && injectorsStr != JSONNull {
		var injectorsObj interface{}
		if err := json.Unmarshal([]byte(injectorsStr), &injectorsObj); err == nil {
			if normalized, err := json.Marshal(injectorsObj); err == nil {
				injectorsStr = string(normalized)
			}
		}
		r.Injectors = tftypes.StringValue(injectorsStr)
	} else {
		r.Injectors = tftypes.StringNull()
	}

	return parseResponseDiags
}
