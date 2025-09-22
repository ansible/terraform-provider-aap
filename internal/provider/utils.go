package provider

import (
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
