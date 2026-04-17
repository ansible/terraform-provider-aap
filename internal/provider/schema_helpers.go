package provider

import (
	"github.com/ansible/terraform-provider-aap/internal/provider/customtypes"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// CommonJobSchemaAttributes returns common schema attributes shared by Job and WorkflowJob resources.
func CommonJobSchemaAttributes(jobTypeName string) map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"job_type": schema.StringAttribute{
			Computed:    true,
			Description: "Job type",
		},
		"url": schema.StringAttribute{
			Computed:    true,
			Description: "URL of the " + jobTypeName,
		},
		"status": schema.StringAttribute{
			Computed:    true,
			Description: "Status of the " + jobTypeName,
		},
		"extra_vars": schema.StringAttribute{
			Description: "Extra Variables. Must be provided as either a JSON or YAML string.",
			Optional:    true,
			CustomType:  customtypes.AAPCustomStringType{},
		},
		"triggers": schema.MapAttribute{
			Optional:    true,
			ElementType: types.StringType,
			Description: "Map of arbitrary keys and values that, when changed, will trigger a creation" +
				" of a new " + jobTypeName + " on AAP. Use 'terraform taint' if you want to force the creation of" +
				" a new " + jobTypeName + " without changing this value.",
		},
		"ignored_fields": schema.ListAttribute{
			ElementType: types.StringType,
			Computed:    true,
			Description: "The list of properties set by the user but ignored on server side.",
		},
		"limit": schema.StringAttribute{
			Description: "Limit pattern to restrict the " + jobTypeName + " run to specific hosts.",
			Optional:    true,
			Computed:    true,
			CustomType:  customtypes.AAPCustomStringType{},
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
		"job_tags": schema.StringAttribute{
			Description: "Tags to include in the " + jobTypeName + " run.",
			Optional:    true,
			Computed:    true,
			CustomType:  customtypes.AAPCustomStringType{},
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
		"skip_tags": schema.StringAttribute{
			Description: "Tags to skip in the " + jobTypeName + " run.",
			Optional:    true,
			Computed:    true,
			CustomType:  customtypes.AAPCustomStringType{},
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
		"labels": schema.ListAttribute{
			Description: "List of label IDs to apply to the " + jobTypeName + ".",
			Optional:    true,
			WriteOnly:   true,
			ElementType: types.Int64Type,
		},
		"wait_for_completion": schema.BoolAttribute{
			Optional: true,
			Computed: true,
			Default:  booldefault.StaticBool(false),
			Description: "When this is set to `true`, Terraform will wait until this aap_job resource is created, reaches " +
				"any final status and then, proceeds with the following resource operation",
		},
		"wait_for_completion_timeout_seconds": schema.Int64Attribute{
			Optional: true,
			Computed: true,
			Default:  int64default.StaticInt64(waitForCompletionTimeoutDefault),
			Description: "Sets the maximum amount of seconds Terraform will wait before timing out the updates, " +
				"and the job creation will fail. Default value of `120`",
		},
	}
}
