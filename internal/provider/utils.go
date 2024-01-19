package provider

import (
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
)

func IsValueProvided(value attr.Value) bool {
	return !value.IsNull() && !value.IsUnknown()
}

func IsResponseValid(resp *http.Response, err error, expected_status int) diag.Diagnostics {
	var diags diag.Diagnostics

	if err != nil {
		diags.AddError("Body JSON Marshal Error", err.Error())
		return diags
	}
	if resp == nil {
		diags.AddError("Http response Error", "no http response from server")
		return diags
	}
	if resp.StatusCode != expected_status {
		diags.AddError("Unexpected Http Status code",
			fmt.Sprintf("expected (%d) got (%s)", expected_status, resp.Status))
		return diags
	}

	return diags
}
