// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package jsontypes

import (
	"context"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/attr/xattr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

var (
	_ basetypes.StringValuable       = (*Exact)(nil)
	_ xattr.ValidateableAttribute    = (*Exact)(nil)
	_ function.ValidateableParameter = (*Exact)(nil)
)

// Exact represents a valid JSON string (RFC 7159). No semantic equality logic is defined for Exact,
// so it will follow Terraform's data-consistency rules for strings, which must match byte-for-byte.
// Consider using Normalized to allow inconsequential differences between JSON strings (whitespace, property order, etc).
type Exact struct {
	basetypes.StringValue
}

// Type returns an ExactType.
func (v Exact) Type(_ context.Context) attr.Type {
	return ExactType{}
}

// Equal returns true if the given value is equivalent.
func (v Exact) Equal(o attr.Value) bool {
	other, ok := o.(Exact)

	if !ok {
		return false
	}

	return v.StringValue.Equal(other.StringValue)
}

// ValidateAttribute implements attribute value validation. This type requires the value provided to be a String
// value that is valid JSON format (RFC 7159).
func (v Exact) ValidateAttribute(ctx context.Context, req xattr.ValidateAttributeRequest, resp *xattr.ValidateAttributeResponse) {
	if v.IsUnknown() || v.IsNull() {
		return
	}

	if ok := json.Valid([]byte(v.ValueString())); !ok {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid JSON String Value",
			"A string value was provided that is not valid JSON string format (RFC 7159).\n\n"+
				"Given Value: "+v.ValueString()+"\n",
		)

		return
	}
}

// ValidateParameter implements provider-defined function parameter value validation. This type requires the value
// provided to be a String value that is valid JSON format (RFC 7159).
func (v Exact) ValidateParameter(ctx context.Context, req function.ValidateParameterRequest, resp *function.ValidateParameterResponse) {
	if v.IsUnknown() || v.IsNull() {
		return
	}

	if ok := json.Valid([]byte(v.ValueString())); !ok {
		resp.Error = function.NewArgumentFuncError(
			req.Position,
			"Invalid JSON String Value: "+
				"A string value was provided that is not valid JSON string format (RFC 7159).\n\n"+
				"Given Value: "+v.ValueString()+"\n",
		)

		return
	}
}

// Unmarshal calls (encoding/json).Unmarshal with the Exact StringValue and `target` input. A null or unknown value will produce an error diagnostic.
// See encoding/json docs for more on usage: https://pkg.go.dev/encoding/json#Unmarshal
func (v Exact) Unmarshal(target any) diag.Diagnostics {
	var diags diag.Diagnostics

	if v.IsNull() {
		diags.Append(diag.NewErrorDiagnostic("Exact JSON Unmarshal Error", "json string value is null"))
		return diags
	}

	if v.IsUnknown() {
		diags.Append(diag.NewErrorDiagnostic("Exact JSON Unmarshal Error", "json string value is unknown"))
		return diags
	}

	err := json.Unmarshal([]byte(v.ValueString()), target)
	if err != nil {
		diags.Append(diag.NewErrorDiagnostic("Exact JSON Unmarshal Error", err.Error()))
	}

	return diags
}

// NewExactNull creates an Exact with a null value. Determine whether the value is null via IsNull method.
func NewExactNull() Exact {
	return Exact{
		StringValue: basetypes.NewStringNull(),
	}
}

// NewExactUnknown creates an Exact with an unknown value. Determine whether the value is unknown via IsUnknown method.
func NewExactUnknown() Exact {
	return Exact{
		StringValue: basetypes.NewStringUnknown(),
	}
}

// NewExactValue creates an Exact with a known value. Access the value via ValueString method.
func NewExactValue(value string) Exact {
	return Exact{
		StringValue: basetypes.NewStringValue(value),
	}
}

// NewExactPointerValue creates an Exact with a null value if nil or a known value. Access the value via ValueStringPointer method.
func NewExactPointerValue(value *string) Exact {
	return Exact{
		StringValue: basetypes.NewStringPointerValue(value),
	}
}
