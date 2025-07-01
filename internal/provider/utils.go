package provider

import (
	"context"
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
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

func ReturnAAPNamedURL(id types.Int64, name types.String, orgName types.String, uri string) (string, error) {
	if IsValueProvided(id) {
		return path.Join(uri, id.String()), nil
	}

	if IsValueProvided(name) && IsValueProvided(orgName) {
		namedUrl := fmt.Sprintf("%s++%s", name.ValueString(), orgName.ValueString())
		return path.Join(uri, namedUrl), nil
	}

	return "", errors.New("invalid lookup parameters")
}

func IsContextActive(operationName string, ctx context.Context, diagnostics diag.Diagnostics) bool {
	if ctx.Err() == nil {
		if diagnostics != nil {
			diagnostics.AddError(
				fmt.Sprintf("Aborting %s operation", operationName),
				"Context is not active, we cannot continue with the execution",
			)
		} else {
			tflog.Error(ctx, fmt.Sprintf("Aborting %s operation. "+
				"Context is not active, we cannot continue with the execution", operationName))
		}
	}
	return ctx.Err() == nil
}

func DoReadPreconditionsMeet(ctx context.Context, resp *datasource.ReadResponse, client ProviderHTTPClient) bool {
	if resp == nil {
		tflog.Error(ctx, "Response not defined, we cannot continue with the execution")
		return false
	}

	// Check that the current context is active
	if !IsContextActive("Read", ctx, resp.Diagnostics) {
		return false
	}

	// Check that the HTTP Client is defined
	if client == nil {
		resp.Diagnostics.AddError(
			"Aborting Read operation",
			"HTTP Client not configured, we cannot continue with the execution",
		)
		return false
	}
	return true
}

func IsValueProvided(value attr.Value) bool {
	return !(value.IsNull() || value.IsUnknown())
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
