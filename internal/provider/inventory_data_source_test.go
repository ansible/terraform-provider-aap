package provider

import (
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

func TestAddHost(t *testing.T) {
	testTable := []struct {
		name     string
		state    inventoryDataSourceModel
		expected inventoryDataSourceModel
	}{
		{
			name: "add new host",
			state: inventoryDataSourceModel{
				ID:     basetypes.NewInt64Value(1),
				Groups: map[string]groupDataSourceModel{},
				Hosts:  map[string]hostDataSourceModel{},
			},
			expected: inventoryDataSourceModel{
				ID: basetypes.NewInt64Value(1),
				Groups: map[string]groupDataSourceModel{
					"db": {
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
				ID: basetypes.NewInt64Value(1),
				Groups: map[string]groupDataSourceModel{
					"running": {
						Hosts:    []string{"sql"},
						Children: []string{},
					},
				},
				Hosts: map[string]hostDataSourceModel{},
			},
			expected: inventoryDataSourceModel{
				ID: basetypes.NewInt64Value(1),
				Groups: map[string]groupDataSourceModel{
					"running": {
						Hosts:    []string{"sql"},
						Children: []string{},
					},
					"db": {
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
				ID: basetypes.NewInt64Value(1),
				Groups: map[string]groupDataSourceModel{
					"db": {
						Hosts:    []string{"sql"},
						Children: []string{},
					},
				},
				Hosts: map[string]hostDataSourceModel{},
			},
			expected: inventoryDataSourceModel{
				ID: basetypes.NewInt64Value(1),
				Groups: map[string]groupDataSourceModel{
					"db": {
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

func TestAddHostVariable(t *testing.T) {
	testTable := []struct {
		name     string
		state    inventoryDataSourceModel
		expected inventoryDataSourceModel
	}{
		{
			name: "add new host var",
			state: inventoryDataSourceModel{
				ID:     basetypes.NewInt64Value(1),
				Groups: map[string]groupDataSourceModel{},
				Hosts:  map[string]hostDataSourceModel{},
			},
			expected: inventoryDataSourceModel{
				ID:     basetypes.NewInt64Value(1),
				Groups: map[string]groupDataSourceModel{},
				Hosts: map[string]hostDataSourceModel{
					"test": {
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
				ID:     basetypes.NewInt64Value(1),
				Groups: map[string]groupDataSourceModel{},
				Hosts: map[string]hostDataSourceModel{
					"test": {
						HostVars: map[string]string{
							"another_var": "another_var_value",
						},
					},
				},
			},
			expected: inventoryDataSourceModel{
				ID:     basetypes.NewInt64Value(1),
				Groups: map[string]groupDataSourceModel{},
				Hosts: map[string]hostDataSourceModel{
					"test": {
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
				ID:     basetypes.NewInt64Value(1),
				Groups: map[string]groupDataSourceModel{},
				Hosts: map[string]hostDataSourceModel{
					"test": {
						HostVars: map[string]string{
							"some_var": "some_initial_var_value",
						},
					},
				},
			},
			expected: inventoryDataSourceModel{
				ID:     basetypes.NewInt64Value(1),
				Groups: map[string]groupDataSourceModel{},
				Hosts: map[string]hostDataSourceModel{
					"test": {
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

func TestMapHosts(t *testing.T) {
	testTable := []struct {
		name     string
		hosts    []ansibleHost
		state    inventoryDataSourceModel
		expected inventoryDataSourceModel
	}{
		{
			name: "case 1",
			hosts: []ansibleHost{
				{
					Name:   "web",
					Groups: []string{"deployer", "front"},
					Variables: map[string]string{
						"framework": "django",
					},
				},
				{
					Name:   "db",
					Groups: []string{"deployer", "database"},
					Variables: map[string]string{
						"server":  "postgresql",
						"version": "14.0.0",
					},
				},
				{
					Name:      "ansible",
					Groups:    []string{},
					Variables: map[string]string{},
				},
			},
			state: inventoryDataSourceModel{
				ID:     basetypes.NewInt64Value(1),
				Groups: nil,
				Hosts:  nil,
			},
			expected: inventoryDataSourceModel{
				ID: basetypes.NewInt64Value(1),
				Groups: map[string]groupDataSourceModel{
					"deployer": {
						Hosts:    []string{"web", "db"},
						Children: []string{},
					},
					"front": {
						Hosts:    []string{"web"},
						Children: []string{},
					},
					"database": {
						Hosts:    []string{"db"},
						Children: []string{},
					},
					"ungrouped": {
						Hosts:    []string{"ansible"},
						Children: []string{},
					},
					"all": {
						Hosts:    []string{},
						Children: []string{"deployer", "front", "database", "ungrouped"},
					},
				},
				Hosts: map[string]hostDataSourceModel{
					"db": {
						HostVars: map[string]string{
							"server":  "postgresql",
							"version": "14.0.0",
						},
					},
					"web": {
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
