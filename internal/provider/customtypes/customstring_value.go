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
	_ = basetypes.StringValuable(&CustomStringValue{})
	_ = basetypes.StringValuableWithSemanticEquals(&CustomStringValue{})
)

// CustomStringValue implements a custom Terraform value.
type CustomStringValue struct {
	basetypes.StringValue
}

// NewCustomStringNull creates a CustomStringValue with a null value. Determine
// whether the value is null via the CustomStringValue type IsNull method.
func NewCustomStringNull() CustomStringValue {
	return CustomStringValue{
		StringValue: basetypes.NewStringNull(),
	}
}

// NewCustomStringUnknown creates a CustomString with an unknown value.
// Determine whether the value is null via the CustomString type IsNull
// method.
func NewCustomStringUnknown() CustomStringValue {
	return CustomStringValue{
		StringValue: basetypes.NewStringUnknown(),
	}
}

// NewCustomStringValue creates a CustomString with a known value. Access
// the value via the CustomStringValue type ValueTime method.
func NewCustomStringValue(value string) CustomStringValue {
	return CustomStringValue{
		StringValue: basetypes.NewStringValue(value),
	}
}

// NewCustomStringPointerValue creates a CustomString with a null value if
// nil or a known value.
func NewCustomStringPointerValue(value *string) CustomStringValue {
	if value == nil {
		return NewCustomStringNull()
	}

	return NewCustomStringValue(*value)
}

// Equal returns true if the given value is equivalent.
func (v CustomStringValue) Equal(o attr.Value) bool {
	other, ok := o.(CustomStringValue)
	if !ok {
		return false
	}

	return v.StringValue.Equal(other.StringValue)
}

// Type returns an instance of the type.
func (v CustomStringValue) Type(_ context.Context) attr.Type {
	return CustomStringType{}
}

func (v CustomStringValue) String() string {
	return "CustomStringValue"
}

// StringSemanticEquals checks if two CustomStringValue objects have
// equivalent values, even if they are not equal.
func (v CustomStringValue) StringSemanticEquals(_ context.Context, newValuable basetypes.StringValuable) (bool, diag.Diagnostics) {
	var diags diag.Diagnostics

	newValue, ok := newValuable.(CustomStringValue)
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
