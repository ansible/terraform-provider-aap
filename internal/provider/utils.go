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

// TODO: Replace ReturnAAPNamedURL with CreateNamedURL during Resource refactor
func ReturnAAPNamedURL(id types.Int64, name types.String, orgName types.String, uri string) (string, error) {
	if id.ValueInt64() != 0 {
		return path.Join(uri, id.String()), nil
	}

	if name.ValueString() != "" && orgName.ValueString() != "" {
		namedUrl := fmt.Sprintf("%s++%s", name.ValueString(), orgName.ValueString())
		return path.Join(uri, namedUrl), nil
	}

	return "", errors.New("invalid lookup parameters")
}

func IsValueProvidedOrPromised(value attr.Value) bool {
	return (!value.IsNull() || value.IsUnknown())
}

func ValidateResponse(resp *http.Response, body []byte, err error, expected_statuses []int) diag.Diagnostics {
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
	if !slices.Contains(expected_statuses, resp.StatusCode) {
		var info map[string]interface{}
		_ = json.Unmarshal(body, &info)
		diags.AddError(
			fmt.Sprintf("Unexpected HTTP status code received for %s request to path %s", resp.Request.Method, resp.Request.URL),
			fmt.Sprintf("Expected one of (%v), got (%d). Response details: %v", expected_statuses, resp.StatusCode, info),
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

func ParseStringValue(description string) types.String {
	if description != "" {
		return types.StringValue(description)
	} else {
		return types.StringNull()
	}
}

func ParseNormalizedValue(variables string) jsontypes.Normalized {
	if variables != "" {
		return jsontypes.NewNormalizedValue(variables)
	} else {
		return jsontypes.NewNormalizedNull()
	}
}

func ParseAAPCustomStringValue(variables string) customtypes.AAPCustomStringValue {
	if variables != "" {
		return customtypes.NewAAPCustomStringValue(variables)
	} else {
		return customtypes.NewAAPCustomStringNull()
	}
}
