package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
)

type warnIfDefaultUsedInt64Modifier struct {
	attributeName  string
	releaseVersion string
}

func (m warnIfDefaultUsedInt64Modifier) Description(_ context.Context) string {
	return fmt.Sprintf("Warns if the default value for '%s' is used during plan.", m.attributeName)
}

func (m warnIfDefaultUsedInt64Modifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m warnIfDefaultUsedInt64Modifier) PlanModifyInt64(_ context.Context, req planmodifier.Int64Request, resp *planmodifier.Int64Response) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		resp.Diagnostics.Append(diag.NewWarningDiagnostic(
			"Default vaule Used",
			fmt.Sprintf("The default value for %s is being used. %s will be a required value in release %s", m.attributeName, m.attributeName, m.releaseVersion),
		))
	}
}

func WarnIfDefaultInt64Used(attributeName string, releaseVersion string) planmodifier.Int64 {
	return warnIfDefaultUsedInt64Modifier{
		attributeName:  attributeName,
		releaseVersion: releaseVersion,
	}
}
