package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

func IsContextActive(operationName string, ctx context.Context, diagnostics *diag.Diagnostics) bool {
	if ctx.Err() != nil {
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
	if !IsContextActive("Read", ctx, &resp.Diagnostics) {
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
