package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
)

func IsValueProvided(value attr.Value) bool {
	return !value.IsNull() && !value.IsUnknown()
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
		err := json.Unmarshal(body, &info)
		if err != nil {
			diags.AddError("Error unmarshaling JSON response body", err.Error())
		}
		diags.AddError(
			fmt.Sprintf("Unexpected HTTP status code received for %s request to path %s", resp.Request.Method, resp.Request.URL),
			fmt.Sprintf("Expected one of (%v), got (%d). Response details: %v", expected_statuses, resp.StatusCode, info),
		)
		return diags
	}

	return diags
}
