package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
)

func IsValueProvided(value attr.Value) bool {
	return !value.IsNull() && !value.IsUnknown()
}
