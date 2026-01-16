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

// EDACredentialTypeResourceModel maps the credential type resource schema to a Go struct.
type EDACredentialTypeResourceModel struct {
	ID          tftypes.Int64  `tfsdk:"id"`
	Name        tftypes.String `tfsdk:"name"`
	Description tftypes.String `tfsdk:"description"`
	Inputs      tftypes.String `tfsdk:"inputs"`
	Injectors   tftypes.String `tfsdk:"injectors"`
}

// EDACredentialTypeAPIModel represents the EDA API model for credential types.
type EDACredentialTypeAPIModel struct {
	ID          int64           `json:"id,omitempty"`
	URL         string          `json:"url,omitempty"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Inputs      json.RawMessage `json:"inputs,omitempty"`
	Injectors   json.RawMessage `json:"injectors,omitempty"`
}

// EDACredentialTypeResource is the resource implementation.
type EDACredentialTypeResource struct {
	client ProviderHTTPClient
}

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource              = &EDACredentialTypeResource{}
	_ resource.ResourceWithConfigure = &EDACredentialTypeResource{}
)

// NewEDACredentialTypeResource is a helper function to simplify the provider implementation.
func NewEDACredentialTypeResource() resource.Resource {
	return &EDACredentialTypeResource{}
}

// Metadata returns the resource type name.
func (r *EDACredentialTypeResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_eda_credential_type"
}

// Configure adds the provider configured client to the resource.
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

// Schema defines the schema for the resource.
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

// Create creates the credential type resource and sets the Terraform state on success.
func (r *EDACredentialTypeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data EDACredentialTypeResourceModel
	var diags diag.Diagnostics

	// Read Terraform plan data into credential type resource model
	diags = req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Generate request body from credential type data
	createRequestBody, diags := data.generateRequestBody()
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	requestData := bytes.NewReader(createRequestBody)

	// Create new credential type in EDA
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

	// Save new credential type data into credential type resource model
	diags = data.parseHTTPResponse(createResponseBody)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated state
	diags = resp.State.Set(ctx, data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Read refreshes the Terraform state with the latest credential type data.
func (r *EDACredentialTypeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data EDACredentialTypeResourceModel
	var diags diag.Diagnostics

	// Read current Terraform state data into credential type resource model
	diags = req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get latest credential type data from EDA
	url := fmt.Sprintf("/api/eda/v1/credential-types/%d/", data.ID.ValueInt64())
	readResponseBody, diags := r.client.Get(url)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save latest credential type data into credential type resource model
	diags = data.parseHTTPResponse(readResponseBody)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated state
	diags = resp.State.Set(ctx, data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update updates the credential type resource and sets the updated Terraform state on success.
func (r *EDACredentialTypeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data EDACredentialTypeResourceModel
	var diags diag.Diagnostics

	// Read Terraform plan data into credential type resource model
	diags = req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Generate request body from credential type data
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

	// Save updated credential type data into credential type resource model
	diags = data.parseHTTPResponse(updateResponseBody)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated state
	diags = resp.State.Set(ctx, data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete deletes the credential type resource.
func (r *EDACredentialTypeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data EDACredentialTypeResourceModel
	var diags diag.Diagnostics

	// Read current Terraform state data into credential type resource model
	diags = req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete credential type from EDA
	url := fmt.Sprintf("/api/eda/v1/credential-types/%d/", data.ID.ValueInt64())
	_, diags = r.client.Delete(url)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// generateRequestBody creates a JSON encoded request body from the credential type resource data.
func (r *EDACredentialTypeResourceModel) generateRequestBody() ([]byte, diag.Diagnostics) {
	var diags diag.Diagnostics

	// Convert credential type resource data to API data model
	credentialType := EDACredentialTypeAPIModel{
		Name:        r.Name.ValueString(),
		Description: r.Description.ValueString(),
	}

	// Handle inputs - convert string to json.RawMessage if not empty, normalize by trimming whitespace
	if !r.Inputs.IsNull() && r.Inputs.ValueString() != "" {
		credentialType.Inputs = json.RawMessage(strings.TrimSpace(r.Inputs.ValueString()))
	}

	// Handle injectors - convert string to json.RawMessage if not empty, normalize by trimming whitespace
	if !r.Injectors.IsNull() && r.Injectors.ValueString() != "" {
		credentialType.Injectors = json.RawMessage(strings.TrimSpace(r.Injectors.ValueString()))
	}

	// Generate JSON encoded request body
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

// parseHTTPResponse updates the credential type resource data from an EDA API response.
func (r *EDACredentialTypeResourceModel) parseHTTPResponse(body []byte) diag.Diagnostics {
	var parseResponseDiags diag.Diagnostics

	// Unmarshal the JSON response
	var apiCredentialType EDACredentialTypeAPIModel
	err := json.Unmarshal(body, &apiCredentialType)
	if err != nil {
		parseResponseDiags.AddError("Error parsing JSON response from EDA", err.Error())
		return parseResponseDiags
	}

	// Map response to the credential type resource schema and update attribute values
	r.ID = tftypes.Int64Value(apiCredentialType.ID)
	r.Name = tftypes.StringValue(apiCredentialType.Name)
	r.Description = ParseStringValue(apiCredentialType.Description)

	// Convert json.RawMessage to string for inputs
	// Treat empty objects {} as null, and normalize JSON to ensure consistent formatting
	inputsStr := strings.TrimSpace(string(apiCredentialType.Inputs))
	if len(inputsStr) > 0 && inputsStr != "{}" && inputsStr != "null" {
		// Re-marshal to normalize JSON formatting (key ordering, whitespace)
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

	// Convert json.RawMessage to string for injectors
	// Treat empty objects {} as null, and normalize JSON to ensure consistent formatting
	injectorsStr := strings.TrimSpace(string(apiCredentialType.Injectors))
	if len(injectorsStr) > 0 && injectorsStr != "{}" && injectorsStr != "null" {
		// Re-marshal to normalize JSON formatting (key ordering, whitespace)
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
