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
	tftypes "github.com/hashicorp/terraform-plugin-framework/types"
)

type EDACredentialResourceModel struct {
	ID               tftypes.Int64  `tfsdk:"id"`
	Name             tftypes.String `tfsdk:"name"`
	Description      tftypes.String `tfsdk:"description"`
	CredentialTypeID tftypes.Int64  `tfsdk:"credential_type_id"`
	OrganizationID   tftypes.Int64  `tfsdk:"organization_id"`
	InputsWO         tftypes.String `tfsdk:"inputs_wo"`
	InputsWOVersion  tftypes.Int64  `tfsdk:"inputs_wo_version"`
}

type EDACredentialAPIModel struct {
	ID             int64  `json:"id,omitempty"`
	URL            string `json:"url,omitempty"`
	Name           string `json:"name"`
	Description    string `json:"description,omitempty"`
	CredentialType *struct {
		ID int64 `json:"id"`
	} `json:"credential_type,omitempty"`
	Organization *struct {
		ID int64 `json:"id"`
	} `json:"organization,omitempty"`
	CredentialTypeID int64           `json:"credential_type_id,omitempty"` // For POST/PATCH
	OrganizationID   int64           `json:"organization_id,omitempty"`    // For POST/PATCH
	Inputs           json.RawMessage `json:"inputs"`
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
				WriteOnly:   true,
				Description: "Write-only credential inputs as JSON string. These values are sent to the API but never stored in Terraform state. Example: jsonencode({\"username\": \"user\", \"password\": \"secret\"})",
			},
			"inputs_wo_version": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
				Description: "Version number for managing credential updates. If not set, the provider will automatically detect changes to inputs_wo using a SHA-256 hash stored in private state. If set manually, you control when the credential is updated by incrementing this value yourself.",
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

	// Handle version management: user-provided version vs automatic hash-based detection
	var versionToSet tftypes.Int64
	if data.InputsWOVersion.IsNull() || data.InputsWOVersion.IsUnknown() {
		// Auto-managed: store hash in private state, start version at 1
		inputsHash, diags := calculateInputsHash(data.InputsWO.ValueString())
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		// Private state requires valid JSON - wrap hash in a JSON object
		hashJSON := fmt.Sprintf(`{"hash":"%s"}`, inputsHash)
		diags = resp.Private.SetKey(ctx, "inputs_wo_hash", []byte(hashJSON))
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		versionToSet = tftypes.Int64Value(1)
	} else {
		// User-managed version, preserve their value
		versionToSet = data.InputsWOVersion
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
	credentialsURL := path.Join(edaEndpoint, "eda-credentials")
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

	// Restore version value (API doesn't return this)
	data.InputsWOVersion = versionToSet

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

	currentInputsWO := data.InputsWO.ValueString()
	currentVersion := data.InputsWOVersion

	url := fmt.Sprintf("/api/eda/v1/eda-credentials/%d/", data.ID.ValueInt64())
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

	data.InputsWO = tftypes.StringValue(currentInputsWO)
	data.InputsWOVersion = currentVersion

	// Copy private state data forward (hash is preserved automatically)
	if hashBytes, diags := req.Private.GetKey(ctx, "inputs_wo_hash"); diags.HasError() {
		resp.Diagnostics.Append(diags...)
	} else if hashBytes != nil {
		diags = resp.Private.SetKey(ctx, "inputs_wo_hash", hashBytes)
		resp.Diagnostics.Append(diags...)
	}

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

	oldHashBytes, getDiags := req.Private.GetKey(ctx, "inputs_wo_hash")
	resp.Diagnostics.Append(getDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	wasAutoManaged := oldHashBytes != nil

	var configModel EDACredentialResourceModel
	configDiags := req.Config.Get(ctx, &configModel)
	resp.Diagnostics.Append(configDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	configVersion := configModel.InputsWOVersion

	// User is manually managing if they set a non-null, non-unknown value in config
	isNowManual := !configVersion.IsNull() && !configVersion.IsUnknown()
	// wasManual if there was NO hash in private state AND version exists in state
	wasManual := !wasAutoManaged && !state.InputsWOVersion.IsNull()

	if wasAutoManaged && isNowManual {
		resp.Diagnostics.AddError(
			"Cannot switch from auto-managed to manual version management",
			"The inputs_wo_version field was previously auto-managed. Once auto-managed, it cannot be switched to manual mode. "+
				"If you need to manually control the version, you must recreate the resource with inputs_wo_version set from the start.",
		)
		return
	}
	if wasManual && !isNowManual {
		resp.Diagnostics.AddError(
			"Cannot switch from manual to auto-managed version management",
			"The inputs_wo_version field was previously manually managed. Once manually managed, it cannot be switched to auto mode. "+
				"If you need auto-managed version control, you must recreate the resource without setting inputs_wo_version.",
		)
		return
	}

	if wasAutoManaged {
		var hashWrapper struct {
			Hash string `json:"hash"`
		}
		if err := json.Unmarshal(oldHashBytes, &hashWrapper); err != nil {
			resp.Diagnostics.AddError(
				"Error reading stored hash from private state",
				fmt.Sprintf("Could not parse hash JSON: %s", err.Error()),
			)
			return
		}
		oldHash := hashWrapper.Hash

		newHash, diags := calculateInputsHash(data.InputsWO.ValueString())
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		if newHash != oldHash {
			data.InputsWOVersion = tftypes.Int64Value(state.InputsWOVersion.ValueInt64() + 1)
			hashJSON := fmt.Sprintf(`{"hash":"%s"}`, newHash)
			diags = resp.Private.SetKey(ctx, "inputs_wo_hash", []byte(hashJSON))
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}
		} else {
			data.InputsWOVersion = state.InputsWOVersion
		}
	}

	updateRequestBody, diags := data.generateRequestBody()
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	requestData := bytes.NewReader(updateRequestBody)

	url := fmt.Sprintf("/api/eda/v1/eda-credentials/%d/", data.ID.ValueInt64())
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

func (r *EDACredentialResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data EDACredentialResourceModel
	var diags diag.Diagnostics

	diags = req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("/api/eda/v1/eda-credentials/%d/", data.ID.ValueInt64())
	_, diags = r.client.Delete(url)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// calculateInputsHash generates a SHA-256 hash of the inputs for change detection.
// Returns a deterministic hex-encoded hash string.
func calculateInputsHash(inputs string) (string, diag.Diagnostics) {
	var diags diag.Diagnostics

	h := sha256.New()
	h.Write([]byte(inputs))
	hash := hex.EncodeToString(h.Sum(nil))

	return hash, diags
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

	// Inputs field is required by the API - default to empty object if not provided
	if !r.InputsWO.IsNull() && r.InputsWO.ValueString() != "" {
		credential.Inputs = json.RawMessage(r.InputsWO.ValueString())
	} else {
		credential.Inputs = json.RawMessage("{}")
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
	r.Name = tftypes.StringValue(apiCredential.Name)
	r.Description = ParseStringValue(apiCredential.Description)

	if apiCredential.CredentialType != nil && apiCredential.CredentialType.ID > 0 {
		r.CredentialTypeID = tftypes.Int64Value(apiCredential.CredentialType.ID)
	} else if apiCredential.CredentialTypeID > 0 {
		r.CredentialTypeID = tftypes.Int64Value(apiCredential.CredentialTypeID)
	} else {
		r.CredentialTypeID = tftypes.Int64Null()
	}

	if apiCredential.Organization != nil && apiCredential.Organization.ID > 0 {
		r.OrganizationID = tftypes.Int64Value(apiCredential.Organization.ID)
	} else if apiCredential.OrganizationID > 0 {
		r.OrganizationID = tftypes.Int64Value(apiCredential.OrganizationID)
	} else {
		r.OrganizationID = tftypes.Int64Null()
	}

	return parseResponseDiags
}
