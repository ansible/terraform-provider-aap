package provider

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"path"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/ansible/terraform-provider-aap/internal/provider/customtypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
)

// operationTimeoutDefault is the default timeout in seconds for delete and update operations when the host is being used by running jobs (HTTP 409 conflicts).
const operationTimeoutDefault int64 = 600

// Retry state constants
const (
	retryStateRetrying = "retrying"
	retryStateSuccess  = "success"
)

// Retry timing constants
const (
	minTimeoutSeconds      = 2
	initialDelaySeconds    = 1
	jitterTimeoutThreshold = 30
	maxJitterSeconds       = 3
)

// Host AAP API model
type HostAPIModel struct {
	InventoryId int64  `json:"inventory"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	URL         string `json:"url,omitempty"`
	Variables   string `json:"variables,omitempty"`
	Enabled     bool   `json:"enabled"`
	Id          int64  `json:"id,omitempty"`
}

// HostResourceModel maps the host resource schema to a Go struct
type HostResourceModel struct {
	InventoryId             types.Int64                      `tfsdk:"inventory_id"`
	Name                    types.String                     `tfsdk:"name"`
	URL                     types.String                     `tfsdk:"url"`
	Description             types.String                     `tfsdk:"description"`
	Variables               customtypes.AAPCustomStringValue `tfsdk:"variables"`
	Groups                  types.Set                        `tfsdk:"groups"`
	Enabled                 types.Bool                       `tfsdk:"enabled"`
	Id                      types.Int64                      `tfsdk:"id"`
	OperationTimeoutSeconds types.Int64                      `tfsdk:"operation_timeout_seconds"`
}

// HostResource is the resource implementation.
type HostResource struct {
	client ProviderHTTPClient
}

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource              = &HostResource{}
	_ resource.ResourceWithConfigure = &HostResource{}
)

// NewHostResource is a helper function to simplify the provider implementation.
func NewHostResource() resource.Resource {
	return &HostResource{}
}

// Metadata returns the resource type name.
func (r *HostResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_host"
}

// Configure adds the provider configured client to the resource
func (r *HostResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// Schema defines the schema for the host resource.
func (r *HostResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"inventory_id": schema.Int64Attribute{
				Required:    true,
				Description: "Inventory id",
			},
			"url": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Description: "URL of the host",
			},
			"id": schema.Int64Attribute{
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
				Description: "Id of the host",
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the host",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Description for the host",
			},
			"variables": schema.StringAttribute{
				Description: "Variables for the host configuration. Must be provided as either a JSON or YAML string.",
				Optional:    true,
				CustomType:  customtypes.AAPCustomStringType{},
			},
			"enabled": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(true),
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
				Description: "Denotes if the host is online and is available",
			},
			"groups": schema.SetAttribute{
				ElementType: types.Int64Type,
				Optional:    true,
				Validators:  []validator.Set{setvalidator.SizeAtLeast(1)},
				Description: "The list of groups to assosicate with a host.",
			},
			"operation_timeout_seconds": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				Default:  int64default.StaticInt64(operationTimeoutDefault),
				Description: "Timeout in seconds for delete and update operations when the host is being used by running jobs (HTTP 409 conflicts)." +
					" Default value is " + strconv.FormatInt(operationTimeoutDefault, 10) + " seconds (" +
					strconv.FormatInt(operationTimeoutDefault/60, 10) + " minutes).",
			},
		},
		Description: "Creates a host." +
			"\n\nThis resource includes built-in retry logic to handle HTTP 409 (Conflict) errors that occur when the host is being used" +
			" by running jobs. When a delete or update operation encounters a 409 error, the provider will automatically retry the operation until either" +
			" (1) The operation succeeds or (2) The configured timeout is reached. The `operation_timeout_seconds` field controls how long the provider will wait" +
			" before giving up. The default timeout is " + strconv.FormatInt(operationTimeoutDefault, 10) + " seconds (" +
			strconv.FormatInt(operationTimeoutDefault/60, 10) + " minutes). You can adjust this value based on your typical job execution times." +
			"\n\nThe retry logic also handles other transient errors (408, 429, 500, 502, 503, 504) and includes" +
			" exponential backoff with jitter to prevent API overload.",
	}
}

// Create creates the host resource and sets the Terraform state on success.
func (r *HostResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data HostResourceModel
	var diags diag.Diagnostics

	// Read Terraform plan data into host resource model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Create request body from host data
	createRequestBody, diags := data.CreateRequestBody()
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	requestData := bytes.NewReader(createRequestBody)

	// Create new host in AAP
	hostsURL := path.Join(r.client.getApiEndpoint(), "hosts")
	createResponseBody, diags := r.client.Create(hostsURL, requestData)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save new host data into host resource model
	diags = data.ParseHttpResponse(createResponseBody)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !data.Groups.IsNull() {
		elements := make([]int64, 0, len(data.Groups.Elements()))
		resp.Diagnostics.Append(data.Groups.ElementsAs(ctx, &elements, false)...)
		if resp.Diagnostics.HasError() {
			return
		}

		url, diags := getURL(data.URL.ValueString(), "groups")
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		resp.Diagnostics.Append(r.AssociateGroups(ctx, elements, url)...)
		if resp.Diagnostics.HasError() {
			return
		}

		groups, diags := r.ReadAssociatedGroups(data)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		resp.Diagnostics.Append(data.UpdateStateWithGroups(ctx, groups)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Read refreshes the Terraform state with the latest host data.
func (r *HostResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data HostResourceModel
	var diags diag.Diagnostics

	// Read current Terraform state data into host resource model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get latest host data from AAP
	readResponseBody, diags := r.client.Get(data.URL.ValueString())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save latest host data into host resource model
	diags = data.ParseHttpResponse(readResponseBody)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	groups, diags := r.ReadAssociatedGroups(data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(data.UpdateStateWithGroups(ctx, groups)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update updates the host resource and sets the updated Terraform state on success.
func (r *HostResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data HostResourceModel
	var diags diag.Diagnostics

	// Read Terraform plan data into host resource model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Create request body from host data
	updateRequestBody, diags := data.CreateRequestBody()
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get timeout value, default to operationTimeoutDefault seconds if not set
	timeout := time.Duration(data.OperationTimeoutSeconds.ValueInt64()) * time.Second

	// Create operation function for retry logic
	updateOperation := func() ([]byte, diag.Diagnostics, int) {
		// Create a fresh reader for each retry attempt to avoid consuming the same reader multiple times
		requestData := bytes.NewReader(updateRequestBody)
		return r.client.UpdateWithStatus(data.URL.ValueString(), requestData)
	}

	// Use StateChangeConf for sophisticated retry logic with exponential backoff
	stateConf := createRetryStateChangeConf(
		updateOperation,
		timeout,
		[]int{http.StatusOK, http.StatusNoContent},
		"update host",
	)

	result, err := stateConf.WaitForStateContext(ctx)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating host",
			fmt.Sprintf("Failed to update host after timeout (%v): %s", timeout, err.Error()),
		)
		return
	}

	// Extract the response body from the successful result
	updateResponseBody := result.([]byte)

	// Save updated host data into host resource model
	diags = data.ParseHttpResponse(updateResponseBody)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(r.HandleGroupAssociation(ctx, data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	groups, diagReadGroups := r.ReadAssociatedGroups(data)
	diags.Append(diagReadGroups...)
	if diags.HasError() {
		return
	}

	resp.Diagnostics.Append(data.UpdateStateWithGroups(ctx, groups)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete deletes the host resource.
func (r *HostResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data HostResourceModel

	// Read current Terraform state data into host resource model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get timeout value, default to operationTimeoutDefault seconds if not set
	timeout := time.Duration(data.OperationTimeoutSeconds.ValueInt64()) * time.Second

	// Create operation function for retry logic
	deleteOperation := func() ([]byte, diag.Diagnostics, int) {
		return r.client.DeleteWithStatus(data.URL.ValueString())
	}

	// Use StateChangeConf for sophisticated retry logic with exponential backoff
	stateConf := createRetryStateChangeConf(
		deleteOperation,
		timeout,
		[]int{http.StatusNoContent, http.StatusAccepted},
		"delete host",
	)

	_, err := stateConf.WaitForStateContext(ctx)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting host",
			fmt.Sprintf("Failed to delete host after timeout (%v): %s", timeout, err.Error()),
		)
		return
	}
}

// CreateRequestBody creates a JSON encoded request body from the host resource data
func (r *HostResourceModel) CreateRequestBody() ([]byte, diag.Diagnostics) {
	// Convert host resource data to API data model
	host := HostAPIModel{
		InventoryId: r.InventoryId.ValueInt64(),
		Name:        r.Name.ValueString(),
		Description: r.Description.ValueString(),
		Variables:   r.Variables.ValueString(),
		Enabled:     r.Enabled.ValueBool(),
	}

	// Create JSON encoded request body
	jsonBody, err := json.Marshal(host)
	if err != nil {
		var diags diag.Diagnostics
		diags.AddError(
			"Error marshaling request body",
			fmt.Sprintf("Could not create request body for host resource, unexpected error: %s", err.Error()),
		)
		return nil, diags
	}

	return jsonBody, nil
}

// ParseHttpResponse updates the host resource data from an AAP API response
func (r *HostResourceModel) ParseHttpResponse(body []byte) diag.Diagnostics {
	var diags diag.Diagnostics

	// Unmarshal the JSON response
	var resultApiHost HostAPIModel
	err := json.Unmarshal(body, &resultApiHost)
	if err != nil {
		diags.AddError("Error parsing JSON response from AAP", err.Error())
		return diags
	}

	// Map response to the host resource schema and update attribute values
	r.InventoryId = types.Int64Value(resultApiHost.InventoryId)
	r.URL = types.StringValue(resultApiHost.URL)
	r.Id = types.Int64Value(resultApiHost.Id)
	r.Name = types.StringValue(resultApiHost.Name)
	r.Enabled = basetypes.NewBoolValue(resultApiHost.Enabled)
	r.Description = ParseStringValue(resultApiHost.Description)
	r.Variables = ParseAAPCustomStringValue(resultApiHost.Variables)

	return diags
}

func extractIDs(data map[string]interface{}) []int64 {
	var ids []int64

	if value, ok := data["results"]; ok {
		for _, v := range value.([]interface{}) {
			group := v.(map[string]interface{})
			if id, ok := group["id"]; ok {
				ids = append(ids, int64(id.(float64)))
			}
		}
	}

	return ids
}

func sliceDifference(slice1 []int64, slice2 []int64) []int64 {
	var difference []int64

	for _, v := range slice1 {
		if !slices.Contains(slice2, v) {
			difference = append(difference, v)
		}
	}
	return difference
}

func (r *HostResource) HandleGroupAssociation(ctx context.Context, data HostResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	elements := make([]int64, 0, len(data.Groups.Elements()))
	diags.Append(data.Groups.ElementsAs(ctx, &elements, false)...)
	if diags.HasError() {
		return diags
	}

	groups, diagReadgroups := r.ReadAssociatedGroups(data)
	diags.Append(diagReadgroups...)
	if diags.HasError() {
		return diags
	}

	toBeAdded := sliceDifference(elements, groups)
	toBeRemoved := sliceDifference(groups, elements)
	url, diags := getURL(data.URL.ValueString(), "groups")
	diags.Append(diags...)
	if diags.HasError() {
		return diags
	}

	if len(toBeAdded) > 0 {
		diags.Append(r.AssociateGroups(ctx, toBeAdded, url)...)
		if diags.HasError() {
			return diags
		}
	}

	if len(toBeRemoved) > 0 {
		diags.Append(r.AssociateGroups(ctx, toBeRemoved, url, true)...)
		if diags.HasError() {
			return diags
		}
	}

	return diags
}

func (r *HostResource) ReadAssociatedGroups(data HostResourceModel) ([]int64, diag.Diagnostics) {
	var diags diag.Diagnostics
	var result map[string]interface{}

	url, diags := getURL(data.URL.ValueString(), "groups")
	diags.Append(diags...)
	if diags.HasError() {
		return nil, diags
	}

	// Get latest host data from AAP
	readResponseBody, diagsGetGroups := r.client.Get(url)
	diags.Append(diagsGetGroups...)
	if diags.HasError() {
		return nil, diags
	}

	/* Unmarshal the json string */
	err := json.Unmarshal(readResponseBody, &result)
	if err != nil {
		diags.AddError("Error parsing JSON response from AAP", err.Error())
		return nil, diags
	}

	return extractIDs(result), diags
}

func (r *HostResourceModel) UpdateStateWithGroups(ctx context.Context, groups []int64) diag.Diagnostics {
	var diags diag.Diagnostics

	convertedGroups, diagConvertToInt64 := types.SetValueFrom(ctx, types.Int64Type, groups)
	diags.Append(diagConvertToInt64...)
	if diags.HasError() {
		return diags
	}
	r.Groups = convertedGroups

	return diags
}

func (r *HostResource) AssociateGroups(ctx context.Context, data []int64, url string, args ...bool) diag.Diagnostics {
	var diags diag.Diagnostics
	var wg sync.WaitGroup
	disassociate := false

	// If disassociate is not provided (zero value), use default value (false)
	if len(args) > 0 {
		disassociate = args[0]
	}

	ctx, cancel := context.WithCancel(context.Background())
	// Make sure it's called to release resources even if no errors
	defer cancel()

	for _, elem := range data {
		wg.Add(1)
		go func(elem int64) {
			defer wg.Done()

			// Check if any error occurred in any other gorouties
			select {
			case <-ctx.Done():
				// Error somewhere, terminate
				return
			default: // Default is must to avoid blocking
			}

			body := make(map[string]int64)
			body["id"] = elem
			if disassociate {
				body["disassociate"] = 1
			}
			json_raw, err := json.Marshal(body)
			if err != nil {
				diags.Append(diag.NewErrorDiagnostic("Body JSON Marshal Error", err.Error()))
				cancel()
				return
			}
			req_data := bytes.NewReader(json_raw)

			resp, bodyreq, err := r.client.doRequest(http.MethodPost, url, req_data)
			diags.Append(ValidateResponse(resp, bodyreq, err, []int{http.StatusNoContent})...)
			if diags.HasError() {
				cancel()
				return
			}
		}(elem)
	}

	// Wait for all goroutines to finish
	wg.Wait()

	if diags.HasError() {
		return diags
	}

	return diags
}

// createRetryStateChangeConf creates a StateChangeConf for retrying operations with exponential backoff.
// This follows Terraform provider best practices for handling transient API errors.
//
// Retryable scenarios based on RFC 7231 and industry standards:
// - HTTP 409: Resource conflict (host in use by running jobs)
// - HTTP 408/429: Client timeouts and rate limiting
// - HTTP 5xx: Server-side transient errors
//
// Uses crypto/rand for jitter to prevent thundering herd in multi-client environments.
func createRetryStateChangeConf(
	operation func() ([]byte, diag.Diagnostics, int),
	timeout time.Duration,
	successStatusCodes []int,
	operationName string,
) *retry.StateChangeConf {
	stateConf := &retry.StateChangeConf{
		Pending: []string{retryStateRetrying},
		Target:  []string{retryStateSuccess},
		Refresh: func() (interface{}, string, error) {
			body, diags, statusCode := operation()

			// Check for retryable status codes
			switch statusCode {
			case http.StatusConflict:
				return nil, retryStateRetrying, nil // Keep retrying
			case http.StatusRequestTimeout, http.StatusTooManyRequests,
				http.StatusInternalServerError, http.StatusBadGateway,
				http.StatusServiceUnavailable, http.StatusGatewayTimeout:
				return nil, retryStateRetrying, nil // Keep retrying
			}

			// Check for success cases
			for _, successCode := range successStatusCodes {
				if statusCode == successCode {
					if diags.HasError() {
						return nil, "", fmt.Errorf("%s succeeded but diagnostics has errors: %v", operationName, diags)
					}
					return body, retryStateSuccess, nil
				}
			}

			// Non-retryable error
			return nil, "", fmt.Errorf("non-retryable HTTP status %d for %s", statusCode, operationName)
		},
		Timeout:    timeout,
		MinTimeout: minTimeoutSeconds * time.Second,   // Minimum wait between retries
		Delay:      initialDelaySeconds * time.Second, // Initial delay before first retry
	}

	// Add jitter to prevent thundering herd - randomize the MinTimeout
	if timeout > jitterTimeoutThreshold*time.Second {
		// For longer timeouts, add more jitter using crypto/rand for better security
		maxJitter := big.NewInt(maxJitterSeconds)
		jitterBig, err := rand.Int(rand.Reader, maxJitter)
		if err == nil {
			jitter := time.Duration(jitterBig.Int64()) * time.Second      // 0-2 seconds
			stateConf.MinTimeout = minTimeoutSeconds*time.Second + jitter // 2-4 seconds total
		}
	}

	return stateConf
}
