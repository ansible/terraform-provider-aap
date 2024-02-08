package provider

import (
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

func TestAddHost(t *testing.T) {
	testTable := []struct {
		name     string
		state    InventoryDataSourceModel
		expected InventoryDataSourceModel
	}{
		{
			name: "add new host",
			state: InventoryDataSourceModel{
				Id:   basetypes.NewInt64Value(1),
				Name: types.StringValue("test inventory"),
			},
			expected: InventoryDataSourceModel{
				Id:   basetypes.NewInt64Value(1),
				Name: types.StringValue("test inventory"),
			},
		},
	}

	for _, tc := range testTable {
		t.Run(tc.name, func(t *testing.T) {
			if !reflect.DeepEqual(tc.state, tc.expected) {
				t.Errorf("expected (%v)", tc.expected)
				t.Errorf("result   (%v)", tc.state)
			}
		})
	}
}
