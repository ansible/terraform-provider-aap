package customtypes

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

var (
	_ = basetypes.StringValuable(&AAPCustomStringValue{})
	_ = basetypes.StringValuableWithSemanticEquals(&AAPCustomStringValue{})
)

// AAPCustomStringValue implements a custom Terraform value.
type AAPCustomStringValue struct {
	basetypes.StringValue
}

// NewAAPCustomStringNull creates a AAPCustomStringValue with a null value. Determine
// whether the value is null via the AAPCustomStringValue type IsNull method.
func NewAAPCustomStringNull() AAPCustomStringValue {
	return AAPCustomStringValue{
		StringValue: basetypes.NewStringNull(),
	}
}

// NewAAPCustomStringUnknown creates a AAPCustomStringValue with an unknown value.
func NewAAPCustomStringUnknown() AAPCustomStringValue {
	return AAPCustomStringValue{
		StringValue: basetypes.NewStringUnknown(),
	}
}

// NewAAPCustomStringValue creates a AAPCustomStringValue with a known value.
func NewAAPCustomStringValue(value string) AAPCustomStringValue {
	return AAPCustomStringValue{
		StringValue: basetypes.NewStringValue(value),
	}
}

// NewAAPCustomStringPointerValue creates a AAPCustomStringValue with a null value if
// nil or a known value.
func NewCustomStringPointerValue(value *string) AAPCustomStringValue {
	if value == nil {
		return NewAAPCustomStringNull()
	}

	return NewAAPCustomStringValue(*value)
}

// Equal returns true if the given value is equivalent.
func (v AAPCustomStringValue) Equal(o attr.Value) bool {
	other, ok := o.(AAPCustomStringValue)
	if !ok {
		return false
	}

	return v.StringValue.Equal(other.StringValue)
}

// Type returns an instance of the type.
func (v AAPCustomStringValue) Type(_ context.Context) attr.Type {
	return AAPCustomStringType{}
}

func (v AAPCustomStringValue) String() string {
	return "AAPCustomStringValue"
}

// StringSemanticEquals checks if two AAPCustomStringValue objects have
// equivalent values, even if they are not equal.
func (v AAPCustomStringValue) StringSemanticEquals(_ context.Context, newValuable basetypes.StringValuable) (bool, diag.Diagnostics) {
	var diags diag.Diagnostics

	newValue, ok := newValuable.(AAPCustomStringValue)
	if !ok {
		diags.AddError(
			"Semantic Equality Check Error",
			fmt.Sprintf("Expected value type %T but got value type %T. Please report this to the provider developers.", v, newValuable),
		)

		return false, diags
	}

	priorValue := v.ValueString()
	currentValue := strings.TrimSpace(newValue.ValueString())

	return priorValue == currentValue, nil
}
