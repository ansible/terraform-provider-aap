package provider

import (
	"context"
	"fmt"
	"slices"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &inventoryDataSource{}
	_ datasource.DataSourceWithConfigure = &inventoryDataSource{}
)

// NewInventoryDataSource is a helper function to simplify the provider implementation.
func NewInventoryDataSource() datasource.DataSource {
	return &inventoryDataSource{}
}

// inventoryDataSource is the data source implementation.
type inventoryDataSource struct {
	client *AAPClient
}

// Metadata returns the data source type name.
func (d *inventoryDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_inventory"
}

// Schema defines the schema for the data source.
func (d *inventoryDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Required: true,
			},
			"groups": schema.MapNestedAttribute{
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"hosts": schema.ListAttribute{
							ElementType: types.StringType,
							Computed:    true,
						},
						"children": schema.ListAttribute{
							ElementType: types.StringType,
							Computed:    true,
						},
					},
				},
				Computed: true,
			},
			"hosts": schema.MapNestedAttribute{
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"hostvars": schema.MapAttribute{
							ElementType: types.StringType,
							Computed:    true,
						},
					},
				},
				Computed: true,
			},
		},
	}
}

const ungroupedName string = "ungrouped"
const allgroupsName string = "all"

// Read refreshes the Terraform state with the latest data.
func (d *inventoryDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state inventoryDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)

	hosts, err := d.client.GetHosts(state.Id.String())
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read Ansible hosts",
			err.Error(),
		)
		return
	}

	// Map response
	state.Groups = make(map[string]groupDataSourceModel)
	state.Hosts = make(map[string]hostDataSourceModel)

	all_groups := []string{}

	for _, host := range hosts.Hosts {
		// add host to group
		if len(host.Groups) == 0 {
			// add host to group name "ungrouped"
			state.addHost(ungroupedName, host.Name)
			// update unique list of groups
			if !slices.Contains(all_groups, ungroupedName) {
				all_groups = append(all_groups, ungroupedName)
			}
		} else {
			for _, group := range host.Groups {
				// add host to new group
				state.addHost(group, host.Name)
				// update unique list of groups
				if !slices.Contains(all_groups, group) {
					all_groups = append(all_groups, group)
				}
			}
		}
		// add host variables
		empty_host := hostDataSourceModel{
			HostVars: make(map[string]string),
		}
		state.Hosts[host.Name] = empty_host
		for key, value := range host.Variables {
			state.addHostVariable(host.Name, key, value)
		}
	}

	// add "all" group
	state.Groups[allgroupsName] = groupDataSourceModel{
		Children: all_groups,
	}

	// Set state
	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Configure adds the provider configured client to the data source.
func (d *inventoryDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*AAPClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *AAPClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	d.client = client
}

// inventoryDataSourceModel maps the data source schema data.
type inventoryDataSourceModel struct {
	Id     types.Int64                     `tfsdk:"id"`
	Groups map[string]groupDataSourceModel `tfsdk:"groups"`
	Hosts  map[string]hostDataSourceModel  `tfsdk:"hosts"`
}

type groupDataSourceModel struct {
	Hosts    []string `tfsdk:"hosts"`
	Children []string `tfsdk:"children"`
}

type hostDataSourceModel struct {
	HostVars map[string]string `tfsdk:"hostvars"`
}

// add host to group
func (d *inventoryDataSourceModel) addHost(groupName string, hostName string) {
	// add host to group
	group_hosts, ok := d.Groups[groupName]
	if !ok {
		group_hosts := new(groupDataSourceModel)
		group_hosts.Hosts = []string{hostName}
		d.Groups[groupName] = *group_hosts
	} else if !slices.Contains(group_hosts.Hosts, hostName) {
		group_hosts.Hosts = append(group_hosts.Hosts, hostName)
		d.Groups[groupName] = group_hosts
	}
}

// add host variables
func (d *inventoryDataSourceModel) addHostVariable(hostName string, varName string, varValue string) {
	_, ok := d.Hosts[hostName]
	if !ok {
		hostvars := new(hostDataSourceModel)
		hostvars.HostVars = make(map[string]string)
		d.Hosts[hostName] = *hostvars
	}
	d.Hosts[hostName].HostVars[varName] = varValue
}
