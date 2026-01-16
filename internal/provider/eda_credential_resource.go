package provider

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	tftypes "github.com/hashicorp/terraform-plugin-framework/types"
)

type EDACredentialResourceModel struct {
	ID               tftypes.Int64  `tfsdk:"id"`
	URL              tftypes.String `tfsdk:"url"`
	Name             tftypes.String `tfsdk:"name"`
	Description      tftypes.String `tfsdk:"description"`
	CredentialTypeID tftypes.Int64  `tfsdk:"credential_type_id"`
	OrganizationID   tftypes.Int64  `tfsdk:"organization_id"`
	InputsWO         tftypes.String `tfsdk:"inputs_wo"`
	InputsWOHash     tftypes.String `tfsdk:"inputs_wo_hash"`
}

type EDACredentialAPIModel struct {
	ID               int64           `json:"id,omitempty"`
	URL              string          `json:"url,omitempty"`
	Name             string          `json:"name"`
	Description      string          `json:"description,omitempty"`
	CredentialTypeID int64           `json:"credential_type_id"`
	OrganizationID   int64           `json:"organization_id,omitempty"`
	Inputs           json.RawMessage `json:"inputs,omitempty"`
}

type EDACredentialResource struct {
	client ProviderHTTPClient
}

var (
	_ resource.Resource              = &EDACredentialResource{}
	_ resource.ResourceWithConfigure = &EDACredentialResource{}
)

func NewEDACredentialResource() resource.Resource {
	return &EDACredentialResource{}
}

func (r *EDACredentialResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_eda_credential"
}

func (r *EDACredentialResource) Configure(_ context.Context, req resource.ConfigureRequest,
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

func (r *EDACredentialResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
				Description: "EDA Credential id",
			},
			"url": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Description: "URL of the EDA Credential",
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the EDA Credential",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Description for the EDA Credential",
			},
			"credential_type_id": schema.Int64Attribute{
				Required:    true,
				Description: "ID of the credential type for this credential",
			},
			"organization_id": schema.Int64Attribute{
				Optional:    true,
				Description: "ID of the organization for this credential",
			},
			"inputs_wo": schema.StringAttribute{
				Required:    true,
				Sensitive:   true,
				Description: "Write-only credential inputs as JSON string. These values are sent to the API but never stored in Terraform state. Example: jsonencode({\"username\": \"user\", \"password\": \"secret\"})",
			},
			"inputs_wo_hash": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Description: "SHA256 hash of inputs_wo used for change detection. Automatically calculated by the provider.",
			},
		},
		Description: `Creates an EDA credential with write-only secret inputs that are never stored in Terraform state.`,
	}
}

func (r *EDACredentialResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data EDACredentialResourceModel
	var diags diag.Diagnostics

	diags = req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	inputsHash := calculateInputsHash(data.InputsWO.ValueString())
	data.InputsWOHash = tftypes.StringValue(inputsHash)

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
	credentialsURL := path.Join(edaEndpoint, "credentials")
	createResponseBody, diags := r.client.Create(credentialsURL, requestData)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = data.parseHTTPResponse(createResponseBody)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.InputsWOHash = tftypes.StringValue(inputsHash)

	diags = resp.State.Set(ctx, data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *EDACredentialResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data EDACredentialResourceModel
	var diags diag.Diagnostics

	diags = req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	currentHash := data.InputsWOHash.ValueString()
	currentInputsWO := data.InputsWO.ValueString()

	readResponseBody, diags := r.client.Get(data.URL.ValueString())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = data.parseHTTPResponse(readResponseBody)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Restore hash and inputs_wo from state since API doesn't return secrets
	data.InputsWOHash = tftypes.StringValue(currentHash)
	data.InputsWO = tftypes.StringValue(currentInputsWO)

	diags = resp.State.Set(ctx, data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *EDACredentialResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data EDACredentialResourceModel
	var state EDACredentialResourceModel
	var diags diag.Diagnostics

	diags = req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	newInputsHash := calculateInputsHash(data.InputsWO.ValueString())
	oldInputsHash := state.InputsWOHash.ValueString()

	if newInputsHash != oldInputsHash {
		data.InputsWOHash = tftypes.StringValue(newInputsHash)
	} else {
		data.InputsWOHash = state.InputsWOHash
	}

	updateRequestBody, diags := data.generateRequestBody()
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	requestData := bytes.NewReader(updateRequestBody)

	updateResponseBody, diags := r.client.Update(data.URL.ValueString(), requestData)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = data.parseHTTPResponse(updateResponseBody)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.InputsWOHash = tftypes.StringValue(newInputsHash)

	diags = resp.State.Set(ctx, data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *EDACredentialResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data EDACredentialResourceModel
	var diags diag.Diagnostics

	diags = req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, diags = r.client.Delete(data.URL.ValueString())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func calculateInputsHash(inputs string) string {
	h := sha256.New()
	h.Write([]byte(inputs))
	return hex.EncodeToString(h.Sum(nil))
}

func (r *EDACredentialResourceModel) generateRequestBody() ([]byte, diag.Diagnostics) {
	var diags diag.Diagnostics

	credential := EDACredentialAPIModel{
		Name:             r.Name.ValueString(),
		Description:      r.Description.ValueString(),
		CredentialTypeID: r.CredentialTypeID.ValueInt64(),
	}

	if !r.OrganizationID.IsNull() && r.OrganizationID.ValueInt64() > 0 {
		credential.OrganizationID = r.OrganizationID.ValueInt64()
	}

	if !r.InputsWO.IsNull() && r.InputsWO.ValueString() != "" {
		credential.Inputs = json.RawMessage(r.InputsWO.ValueString())
	}

	jsonBody, err := json.Marshal(credential)
	if err != nil {
		diags.AddError(
			"Error marshaling request body",
			fmt.Sprintf("Could not generate request body for EDA credential resource, unexpected error: %s", err.Error()),
		)
		return nil, diags
	}

	return jsonBody, nil
}

func (r *EDACredentialResourceModel) parseHTTPResponse(body []byte) diag.Diagnostics {
	var parseResponseDiags diag.Diagnostics

	var apiCredential EDACredentialAPIModel
	err := json.Unmarshal(body, &apiCredential)
	if err != nil {
		parseResponseDiags.AddError("Error parsing JSON response from EDA", err.Error())
		return parseResponseDiags
	}

	r.ID = tftypes.Int64Value(apiCredential.ID)
	r.URL = tftypes.StringValue(apiCredential.URL)
	r.Name = tftypes.StringValue(apiCredential.Name)
	r.Description = ParseStringValue(apiCredential.Description)
	r.CredentialTypeID = tftypes.Int64Value(apiCredential.CredentialTypeID)
	
	if apiCredential.OrganizationID > 0 {
		r.OrganizationID = tftypes.Int64Value(apiCredential.OrganizationID)
	} else {
		r.OrganizationID = tftypes.Int64Null()
	}

	return parseResponseDiags
}
