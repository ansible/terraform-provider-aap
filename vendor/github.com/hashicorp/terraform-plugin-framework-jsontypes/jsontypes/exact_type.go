// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package jsontypes

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var (
	_ basetypes.StringTypable = (*ExactType)(nil)
)

// ExactType is an attribute type that represents a valid JSON string (RFC 7159). No semantic equality logic is defined for ExactType,
// so it will follow Terraform's data-consistency rules for strings, which must match byte-for-byte. Consider using NormalizedType
// to allow inconsequential differences between JSON strings (whitespace, property order, etc).
type ExactType struct {
	basetypes.StringType
}

// String returns a human readable string of the type name.
func (t ExactType) String() string {
	return "jsontypes.ExactType"
}

// ValueType returns the Value type.
func (t ExactType) ValueType(ctx context.Context) attr.Value {
	return Exact{}
}

// Equal returns true if the given type is equivalent.
func (t ExactType) Equal(o attr.Type) bool {
	other, ok := o.(ExactType)

	if !ok {
		return false
	}

	return t.StringType.Equal(other.StringType)
}

// ValueFromString returns a StringValuable type given a StringValue.
func (t ExactType) ValueFromString(ctx context.Context, in basetypes.StringValue) (basetypes.StringValuable, diag.Diagnostics) {
	return Exact{
		StringValue: in,
	}, nil
}

// ValueFromTerraform returns a Value given a tftypes.Value.  This is meant to convert the tftypes.Value into a more convenient Go type
// for the provider to consume the data with.
func (t ExactType) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	attrValue, err := t.StringType.ValueFromTerraform(ctx, in)
	if err != nil {
		return nil, err
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
