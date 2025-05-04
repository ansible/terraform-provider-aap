package provider

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strings"

	"github.com/ansible/terraform-provider-aap/internal/provider/customtypes"
	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// type AAPDataSourceModel interface {
// 	types.Int64
// 	types.String
// 	types.String
// }

type AAPDataSource[T any] struct {
	client ProviderHTTPClient
}

type AAPDataSourceModel[T any] struct {
	Id               types.Int64
	Name             types.String
	OrganizationName types.String
}

func NewAAPDataSourceModel(model any) AAPDataSourceModel[any] {
	return *&AAPDataSourceModel[any]{}
}

func (dsm *AAPDataSourceModel[T]) ReturnAAPResourceUrlDataSourceModel(datasource AAPDataSource[T]) (string, error) {
	if !dsm.Id.IsNull() {
		return path.Join(datasource.client.getApiEndpoint(), "inventories", dsm.Id.String()), nil
	} else if !dsm.Name.IsNull() && !dsm.OrganizationName.IsNull() {
		namedUrl := strings.Join([]string{dsm.Name.String()[1 : len(dsm.Name.String())-1], "++", dsm.OrganizationName.String()[1 : len(dsm.OrganizationName.String())-1]}, "")
		return path.Join(datasource.client.getApiEndpoint(), "inventories", namedUrl), nil
	} else {
		return types.StringNull().String(), errors.New("invalid lookup parameters")
	}
}

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

type ParseValue interface {
	ParseValue(value string) any
}

type StringTyped struct {
}

func (t *StringTyped) ParseValue(value string) types.String {
	if value != "" {
		return types.StringValue(value)
	} else {
		return types.StringNull()
	}
}

// func (t *TypedParseValue) ParseValue(value string) jsontypes.Normalized {

// }

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
