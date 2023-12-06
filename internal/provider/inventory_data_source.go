package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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

	hosts, err := d.ReadAnsibleHosts(state.ID.String())
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read Ansible hosts",
			err.Error(),
		)
		return
	}

	state.mapHosts(hosts.Hosts)

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
	ID     types.Int64                     `tfsdk:"id"`
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

func (d *inventoryDataSourceModel) mapHosts(hosts []ansibleHost) {
	d.Groups = make(map[string]groupDataSourceModel)
	d.Hosts = make(map[string]hostDataSourceModel)

	allGroups := []string{}

	for _, host := range hosts {
		// add host to group
		if len(host.Groups) == 0 {
			// add host to group name "ungrouped"
			d.addHost(ungroupedName, host.Name)
			// update unique list of groups
			if !slices.Contains(allGroups, ungroupedName) {
				allGroups = append(allGroups, ungroupedName)
			}
		} else {
			for _, group := range host.Groups {
				// add host to new group
				d.addHost(group, host.Name)
				// update unique list of groups
				if !slices.Contains(allGroups, group) {
					allGroups = append(allGroups, group)
				}
			}
		}
		// add host variables
		if len(host.Variables) > 0 {
			emptyHost := hostDataSourceModel{
				HostVars: make(map[string]string),
			}
			d.Hosts[host.Name] = emptyHost
			for key, value := range host.Variables {
				d.addHostVariable(host.Name, key, value)
			}
		}
	}

	// add "all" group
	d.Groups[allgroupsName] = groupDataSourceModel{
		Hosts:    []string{},
		Children: allGroups,
	}
}

// add host to group
func (d *inventoryDataSourceModel) addHost(groupName string, hostName string) {
	// add host to group
	groupHosts, ok := d.Groups[groupName]
	if !ok {
		groupHosts := &groupDataSourceModel{
			Hosts:    []string{},
			Children: []string{},
		}
		groupHosts.Hosts = append(groupHosts.Hosts, hostName)
		d.Groups[groupName] = *groupHosts
	} else if !slices.Contains(groupHosts.Hosts, hostName) {
		groupHosts.Hosts = append(groupHosts.Hosts, hostName)
		d.Groups[groupName] = groupHosts
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

// ansible host
type ansibleHost struct {
	Name      string            `json:"name"`
	Groups    []string          `json:"groups"`
	Variables map[string]string `json:"variables"`
}

// ansible host list
type ansibleHostList struct {
	Hosts []ansibleHost `json:"hosts"`
}

func (d *inventoryDataSource) ReadAnsibleHosts(stateID string) (*ansibleHostList, error) {
	httpStatusCode, body, err := d.client.doRequest("GET", "api/v2/state/"+stateID+"/", nil)
	if err != nil {
		return nil, err
	}
	if httpStatusCode != http.StatusOK {
		return nil, fmt.Errorf("status: %d, body: %s", httpStatusCode, body)
	}

	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, err
	}

	var hosts ansibleHostList
	resources, ok := result["resources"].([]interface{})
	if ok {
		for _, resource := range resources {
			resourceObj := resource.(map[string]interface{})
			resourceType, ok := resourceObj["type"]
			if ok && resourceType == "ansible_host" {
				instances, ok := resourceObj["instances"].([]interface{})
				if ok {
					for _, instance := range instances {
						attributes, ok := instance.(map[string]interface{})["attributes"].(map[string]interface{})
						if ok {
							name := attributes["name"].(string)
							var groups []string
							for _, group := range attributes["groups"].([]interface{}) {
								groups = append(groups, group.(string))
							}
							variables := make(map[string]string)
							for key, value := range attributes["variables"].(map[string]interface{}) {
								variables[key] = value.(string)
							}
							hosts.Hosts = append(hosts.Hosts, ansibleHost{
								Name:      name,
								Groups:    groups,
								Variables: variables,
							})
						}
					}
				}
			}
		}
	}
	return &hosts, nil
}
