package customtypes

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var (
	_ = basetypes.StringTypable(&AAPCustomStringType{})
)

// AAPCustomStringType implements a custom Terraform type.
type AAPCustomStringType struct {
	basetypes.StringType
}

// / Equal returns true if the given type is equivalent.
func (t AAPCustomStringType) Equal(o attr.Type) bool {
	other, ok := o.(AAPCustomStringType)
	if !ok {
		return false
	}

	return t.StringType.Equal(other.StringType)
}

// String returns a human readable string of the type name.
func (t AAPCustomStringType) String() string {
	return "customtypes.AAPCustomStringType"
}

// / ValueFromString returns a StringValuable type given a StringValue.
func (t AAPCustomStringType) ValueFromString(_ context.Context, in basetypes.StringValue) (basetypes.StringValuable, diag.Diagnostics) {
	value := AAPCustomStringValue{
		StringValue: in,
	}

	return value, nil
}

// ValueFromTerraform converts a Terraform value to a AAPCustomStringValue.
func (t AAPCustomStringType) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	attrValue, err := t.StringType.ValueFromTerraform(ctx, in)
	if err != nil {
		return nil, fmt.Errorf("unexpected error converting value from Terraform: %w", err)
	}

	stringValue, ok := attrValue.(basetypes.StringValue)
	if !ok {
		return nil, fmt.Errorf("unexpected value type of %T", attrValue)
	}

	stringValuable, diags := t.ValueFromString(ctx, stringValue)
	if diags.HasError() {
		return nil, fmt.Errorf("unexpected error converting StringValue to StringValuable: %v", diags)
	}

	return stringValuable, nil
}

// ValueType returns an instance of the value.
func (t AAPCustomStringType) ValueType(_ context.Context) attr.Value {
	return AAPCustomStringValue{}
}

// Validate implements type validation. This type requires the value provided to be a String value.
func (t AAPCustomStringType) Validate(_ context.Context, in tftypes.Value, path path.Path) diag.Diagnostics {
	var diags diag.Diagnostics

	if in.Type() == nil {
		return diags
	}

	if !in.Type().Is(tftypes.String) {
		err := fmt.Errorf("expected String value, received %T with value: %v", in, in)
		diags.AddAttributeError(
			path,
			"AAPCustomString Type Validation Error",
			"An unexpected error was encountered trying to validate an attribute value. This is always an error in the provider. "+
				"Please report the following to the provider developer:\n\n"+err.Error(),
		)
		return diags
	}

	if !in.IsKnown() || in.IsNull() {
		return diags
	}

	var valueString string

	if err := in.As(&valueString); err != nil {
		diags.AddAttributeError(
			path,
			"AAPCustomString Type Validation Error",
			"An unexpected error was encountered trying to validate an attribute value. This is always an error in the provider. "+
				"Please report the following to the provider developer:\n\n"+err.Error(),
		)

		return diags
	}

	return diags
}
