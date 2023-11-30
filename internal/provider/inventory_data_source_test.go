package provider

import (
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

func TestInventoryDataSourceModelAddHost(t *testing.T) {
	testTable := []struct {
		name     string
		state    inventoryDataSourceModel
		expected inventoryDataSourceModel
	}{
		{
			name: "add new host",
			state: inventoryDataSourceModel{
				Id:     basetypes.NewInt64Value(1),
				Groups: map[string]groupDataSourceModel{},
				Hosts:  map[string]hostDataSourceModel{},
			},
			expected: inventoryDataSourceModel{
				Id: basetypes.NewInt64Value(1),
				Groups: map[string]groupDataSourceModel{
					"db": groupDataSourceModel{
						Hosts:    []string{"sql"},
						Children: []string{},
					},
				},
				Hosts: map[string]hostDataSourceModel{},
			},
		},
		{
			name: "add existing host into another group",
			state: inventoryDataSourceModel{
				Id: basetypes.NewInt64Value(1),
				Groups: map[string]groupDataSourceModel{
					"running": groupDataSourceModel{
						Hosts:    []string{"sql"},
						Children: []string{},
					},
				},
				Hosts: map[string]hostDataSourceModel{},
			},
			expected: inventoryDataSourceModel{
				Id: basetypes.NewInt64Value(1),
				Groups: map[string]groupDataSourceModel{
					"running": groupDataSourceModel{
						Hosts:    []string{"sql"},
						Children: []string{},
					},
					"db": groupDataSourceModel{
						Hosts:    []string{"sql"},
						Children: []string{},
					},
				},
				Hosts: map[string]hostDataSourceModel{},
			},
		},
		{
			name: "add duplicate host name into group",
			state: inventoryDataSourceModel{
				Id: basetypes.NewInt64Value(1),
				Groups: map[string]groupDataSourceModel{
					"db": groupDataSourceModel{
						Hosts:    []string{"sql"},
						Children: []string{},
					},
				},
				Hosts: map[string]hostDataSourceModel{},
			},
			expected: inventoryDataSourceModel{
				Id: basetypes.NewInt64Value(1),
				Groups: map[string]groupDataSourceModel{
					"db": groupDataSourceModel{
						Hosts:    []string{"sql"},
						Children: []string{},
					},
				},
				Hosts: map[string]hostDataSourceModel{},
			},
		},
	}
	for _, tc := range testTable {
		t.Run(tc.name, func(t *testing.T) {
			tc.state.addHost("db", "sql")
			if !reflect.DeepEqual(tc.state, tc.expected) {
				t.Errorf("expected (%v)", tc.expected)
				t.Errorf("result   (%v)", tc.state)
			}
		})
	}
}

func TestInventoryDataSourceModelAddHostVariable(t *testing.T) {
	testTable := []struct {
		name     string
		state    inventoryDataSourceModel
		expected inventoryDataSourceModel
	}{
		{
			name: "add new host var",
			state: inventoryDataSourceModel{
				Id:     basetypes.NewInt64Value(1),
				Groups: map[string]groupDataSourceModel{},
				Hosts:  map[string]hostDataSourceModel{},
			},
			expected: inventoryDataSourceModel{
				Id:     basetypes.NewInt64Value(1),
				Groups: map[string]groupDataSourceModel{},
				Hosts: map[string]hostDataSourceModel{
					"test": hostDataSourceModel{
						HostVars: map[string]string{
							"some_var": "some_var_value",
						},
					},
				},
			},
		},
		{
			name: "add new var into existing host var",
			state: inventoryDataSourceModel{
				Id:     basetypes.NewInt64Value(1),
				Groups: map[string]groupDataSourceModel{},
				Hosts: map[string]hostDataSourceModel{
					"test": hostDataSourceModel{
						HostVars: map[string]string{
							"another_var": "another_var_value",
						},
					},
				},
			},
			expected: inventoryDataSourceModel{
				Id:     basetypes.NewInt64Value(1),
				Groups: map[string]groupDataSourceModel{},
				Hosts: map[string]hostDataSourceModel{
					"test": hostDataSourceModel{
						HostVars: map[string]string{
							"another_var": "another_var_value",
							"some_var":    "some_var_value",
						},
					},
				},
			},
		},
		{
			name: "override host variable",
			state: inventoryDataSourceModel{
				Id:     basetypes.NewInt64Value(1),
				Groups: map[string]groupDataSourceModel{},
				Hosts: map[string]hostDataSourceModel{
					"test": hostDataSourceModel{
						HostVars: map[string]string{
							"some_var": "some_intial_var_value",
						},
					},
				},
			},
			expected: inventoryDataSourceModel{
				Id:     basetypes.NewInt64Value(1),
				Groups: map[string]groupDataSourceModel{},
				Hosts: map[string]hostDataSourceModel{
					"test": hostDataSourceModel{
						HostVars: map[string]string{
							"some_var": "some_var_value",
						},
					},
				},
			},
		},
	}
	for _, tc := range testTable {
		t.Run(tc.name, func(t *testing.T) {
			tc.state.addHostVariable("test", "some_var", "some_var_value")
			if !reflect.DeepEqual(tc.state, tc.expected) {
				t.Errorf("expected (%v)", tc.expected)
				t.Errorf("result   (%v)", tc.state)
			}
		})
	}
}

func TestInventoryDataSourceModelMapHosts(t *testing.T) {
	testTable := []struct {
		name     string
		hosts    []ansibleHost
		state    inventoryDataSourceModel
		expected inventoryDataSourceModel
	}{
		{
			name: "case 1",
			hosts: []ansibleHost{
				ansibleHost{
					Name:   "web",
					Groups: []string{"deployer", "front"},
					Variables: map[string]string{
						"framework": "django",
					},
				},
				ansibleHost{
					Name:   "db",
					Groups: []string{"deployer", "database"},
					Variables: map[string]string{
						"server":  "postgresql",
						"version": "14.0.0",
					},
				},
				ansibleHost{
					Name:      "ansible",
					Groups:    []string{},
					Variables: map[string]string{},
				},
			},
			state: inventoryDataSourceModel{
				Id:     basetypes.NewInt64Value(1),
				Groups: nil,
				Hosts:  nil,
			},
			expected: inventoryDataSourceModel{
				Id: basetypes.NewInt64Value(1),
				Groups: map[string]groupDataSourceModel{
					"deployer": groupDataSourceModel{
						Hosts:    []string{"web", "db"},
						Children: []string{},
					},
					"front": groupDataSourceModel{
						Hosts:    []string{"web"},
						Children: []string{},
					},
					"database": groupDataSourceModel{
						Hosts:    []string{"db"},
						Children: []string{},
					},
					"ungrouped": groupDataSourceModel{
						Hosts:    []string{"ansible"},
						Children: []string{},
					},
					"all": groupDataSourceModel{
						Hosts:    []string{},
						Children: []string{"deployer", "front", "database", "ungrouped"},
					},
				},
				Hosts: map[string]hostDataSourceModel{
					"db": hostDataSourceModel{
						HostVars: map[string]string{
							"server":  "postgresql",
							"version": "14.0.0",
						},
					},
					"web": hostDataSourceModel{
						HostVars: map[string]string{
							"framework": "django",
						},
					},
				},
			},
		},
	}
	for _, tc := range testTable {
		t.Run(tc.name, func(t *testing.T) {
			tc.state.mapHosts(tc.hosts)
			if !reflect.DeepEqual(tc.state, tc.expected) {
				t.Errorf("expected (%v)", tc.expected)
				t.Errorf("result   (%v)", tc.state)
			}
		})
	}
}
