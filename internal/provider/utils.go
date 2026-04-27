package provider

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"slices"

	"github.com/ansible/terraform-provider-aap/internal/provider/customtypes"
	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ReturnAAPNamedURL returns an AAP named URL for the given model and URI.
// TODO: Replace ReturnAAPNamedURL with CreateNamedURL during Resource refactor
func ReturnAAPNamedURL(id types.Int64, name types.String, orgName types.String, uri string) (string, error) {
	if id.ValueInt64() != 0 {
		return path.Join(uri, id.String()), nil
	}

	if name.ValueString() != "" && orgName.ValueString() != "" {
		namedURL := fmt.Sprintf("%s++%s", name.ValueString(), orgName.ValueString())
		return path.Join(uri, namedURL), nil
	}

	return "", errors.New("invalid lookup parameters")
}

// IsValueProvidedOrPromised checks if a Terraform attribute value is provided or promised.
func IsValueProvidedOrPromised(value attr.Value) bool {
	return (!value.IsNull() || value.IsUnknown())
}

// ValidateResponse validates an HTTP response against expected status codes and returns diagnostics.
func ValidateResponse(resp *http.Response, body []byte, err error, expectedStatuses []int) diag.Diagnostics {
	var diags diag.Diagnostics

	if err != nil {
		diags.AddError(
			"Client request error",
			err.Error(),
		)
		return diags
	}
	if resp == nil {
		diags.AddError("HTTP response error", "No HTTP response from server")
		return diags
	}
	if !slices.Contains(expectedStatuses, resp.StatusCode) {
		var info map[string]interface{}
		_ = json.Unmarshal(body, &info)
		diags.AddError(
			fmt.Sprintf("Unexpected HTTP status code received for %s request to path %s", resp.Request.Method, resp.Request.URL),
			fmt.Sprintf("Expected one of (%v), got (%d). Response details: %v", expectedStatuses, resp.StatusCode, info),
		)
		return diags
	}

	return diags
}

func getURL(base string, paths ...string) (string, diag.Diagnostics) {
	var diags diag.Diagnostics
	u, err := url.ParseRequestURI(base)
	if err != nil {
		diags.AddError("Error parsing the URL", err.Error())
		return "", diags
	}

	u.Path = path.Join(append([]string{u.Path}, paths...)...)

	return u.String(), diags
}

// ParseStringValue parses a string description into a Terraform types.String value.
func ParseStringValue(description string) types.String {
	if description != "" {
		return types.StringValue(description)
	}
	return types.StringNull()
}

// ParseNormalizedValue parses a variables string into a jsontypes.Normalized value.
func ParseNormalizedValue(variables string) jsontypes.Normalized {
	if variables != "" {
		return jsontypes.NewNormalizedValue(variables)
	}
	return jsontypes.NewNormalizedNull()
}

// ParseAAPCustomStringValue parses a variables string into a customtypes.AAPCustomStringValue.
func ParseAAPCustomStringValue(variables string) customtypes.AAPCustomStringValue {
	if variables != "" {
		return customtypes.NewAAPCustomStringValue(variables)
	}
	return customtypes.NewAAPCustomStringNull()
}

// ConvertListToInt64Slice converts a types.List of Int64 to []int64.
// This is used for API fields that expect a simple array of integers, such as instance_groups.
func ConvertListToInt64Slice(list types.List) []int64 {
	if list.IsNull() || list.IsUnknown() {
		return nil
	}

	elements := list.Elements()
	if len(elements) == 0 {
		return nil
	}

	result := make([]int64, 0, len(elements))
	for _, elem := range elements {
		if int64Val, ok := elem.(types.Int64); ok && !int64Val.IsNull() && !int64Val.IsUnknown() {
			result = append(result, int64Val.ValueInt64())
		}
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

// LaunchFieldValidation represents a single field validation for launch-time parameters.
type LaunchFieldValidation struct {
	AskOnLaunch bool
	Value       attr.Value
	FieldName   string
}

// LaunchRequirements contains fields from AAP's /launch/ endpoint that indicate what's required.
type LaunchRequirements struct {
	VariablesNeededToStart  []string
	InventoryNeededToStart  bool
	CredentialNeededToStart bool
}

// extractExtraVarsString extracts the string value from extra_vars attr.Value.
func extractExtraVarsString(extraVarsValue attr.Value) string {
	if extraVarsValue.IsNull() || extraVarsValue.IsUnknown() {
		return ""
	}

	// Handle types.String and customtypes.AAPCustomStringValue via ValueString() interface
	if stringer, ok := extraVarsValue.(interface{ ValueString() string }); ok {
		return stringer.ValueString()
	}
	return ""
}

// validateSurveyVariables validates that required survey variables are provided in extra_vars.
func validateSurveyVariables(requirements LaunchRequirements, extraVarsValue attr.Value, templateType string) diag.Diagnostics {
	var diags diag.Diagnostics

	if len(requirements.VariablesNeededToStart) == 0 {
		return diags
	}

	extraVarsStr := extractExtraVarsString(extraVarsValue)
	var extraVarsMap map[string]interface{}

	if extraVarsStr != "" {
		if err := json.Unmarshal([]byte(extraVarsStr), &extraVarsMap); err != nil {
			diags.AddWarning(
				"Unable to validate required survey variables",
				fmt.Sprintf("Could not parse extra_vars as JSON: %s. Survey variable validation will be performed by AAP.", err.Error()),
			)
			return diags
		}
	}

	for _, varName := range requirements.VariablesNeededToStart {
		if _, exists := extraVarsMap[varName]; !exists {
			diags.AddError(
				"Missing required field",
				fmt.Sprintf("%s requires survey variable '%s' to be provided in extra_vars", templateType, varName),
			)
		}
	}

	return diags
}

// ValidateLaunchFields validates that required fields are provided and warns about ignored fields.
// Used by both Job and WorkflowJob launch validation.
func ValidateLaunchFields(
	requirements LaunchRequirements,
	validations []LaunchFieldValidation,
	templateType string,
	extraVarsValue attr.Value,
) diag.Diagnostics {
	var diags diag.Diagnostics

	// Validate template fields using explicit boolean flags
	for _, v := range validations {
		isNullOrUnknown := v.Value.IsNull() || v.Value.IsUnknown()

		isRequired := (v.FieldName == "inventory_id" && requirements.InventoryNeededToStart) ||
			(v.FieldName == "credentials" && requirements.CredentialNeededToStart)

		if isRequired && isNullOrUnknown {
			diags.AddError(
				"Missing required field",
				fmt.Sprintf("%s requires '%s' to be provided at launch", templateType, v.FieldName),
			)
		}

		if !v.AskOnLaunch && !isNullOrUnknown {
			diags.AddWarning(
				"Field will be ignored",
				fmt.Sprintf("'%s' is provided but the %s does not allow it to be specified at launch", v.FieldName, templateType),
			)
		}
	}

	// Validate survey variables
	diags.Append(validateSurveyVariables(requirements, extraVarsValue, templateType)...)

	return diags
}

// ParseIgnoredFieldsToList converts ignored fields from the AAP API response into a types.List.
// This is shared logic used by Job and WorkflowJob resources.
func ParseIgnoredFieldsToList(ignoredFields map[string]interface{}) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics

	if len(ignoredFields) == 0 {
		return types.ListNull(types.StringType), diags
	}

	var keysList = []attr.Value{}
	for k := range ignoredFields {
		key := k
		if v, ok := keyMapping[k]; ok {
			key = v
		}
		keysList = append(keysList, types.StringValue(key))
	}

	if len(keysList) == 0 {
		return types.ListNull(types.StringType), diags
	}

	list, listDiags := types.ListValue(types.StringType, keysList)
	diags.Append(listDiags...)
	return list, diags
}

// LaunchableJob is an interface for job types that can be launched (Job or WorkflowJob).
type LaunchableJob interface {
	CreateRequestBody() ([]byte, diag.Diagnostics)
	GetTemplateID() int64
}

// GetLaunchConfiguration performs a GET request to retrieve the launch configuration
// for a job or workflow job template.
func GetLaunchConfiguration(
	client ProviderHTTPClient,
	templateType string,
	templateID int64,
	result interface{},
	templateTypeName string,
) diag.Diagnostics {
	var diags diag.Diagnostics
	launchURL := path.Join(client.getAPIEndpoint(), templateType, fmt.Sprintf("%d", templateID), "launch")

	getResp, getBody, getErr := client.doRequest(http.MethodGet, launchURL, nil, nil)
	diags.Append(ValidateResponse(getResp, getBody, getErr, []int{http.StatusOK})...)
	if diags.HasError() {
		return diags
	}

	err := json.Unmarshal(getBody, result)
	if err != nil {
		diags.AddError(
			fmt.Sprintf("Error parsing %s launch configuration", templateTypeName),
			fmt.Sprintf("Could not parse launch configuration response: %s", err.Error()),
		)
		return diags
	}

	return diags
}

// LaunchJobTemplate performs the common POST request to launch a job or workflow job.
// It takes the template type ("job_templates" or "workflow_job_templates") and the model.
func LaunchJobTemplate(
	client ProviderHTTPClient,
	templateType string,
	model LaunchableJob,
) ([]byte, diag.Diagnostics) {
	var diags diag.Diagnostics

	// Create request body
	requestBody, diagCreateReq := model.CreateRequestBody()
	diags.Append(diagCreateReq...)
	if diags.HasError() {
		return nil, diags
	}

	// POST to launch endpoint
	requestData := bytes.NewReader(requestBody)
	postURL := path.Join(client.getAPIEndpoint(), templateType, fmt.Sprintf("%d", model.GetTemplateID()), "launch")
	resp, body, err := client.doRequest(http.MethodPost, postURL, nil, requestData)
	diags.Append(ValidateResponse(resp, body, err, []int{http.StatusCreated})...)
	if diags.HasError() {
		return nil, diags
	}

	return body, diags
}
