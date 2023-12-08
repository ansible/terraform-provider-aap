package provider

import (
        "bytes"
	"context"
	"encoding/json"
	"fmt"
        "net/http"
	"github.com/hashicorp/terraform-plugin-framework/attr"
        "github.com/hashicorp/terraform-plugin-framework/resource"
        "github.com/hashicorp/terraform-plugin-framework/resource/schema"
        "github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure the implementation satisfies the expected interfaces.
var (
    _ resource.Resource              = &groupResource{}
)

// NewGroupResource is a helper function to simplify the provider implementation.
func NewGroupResource() resource.Resource {
    return &groupResource{}
}

type GroupResourceModelInterface interface {
	ParseHttpResponse(body []byte) error
	CreateRequestBody() (*bytes.Reader, error)
}

// groupResource is the resource implementation.
type groupResource struct {
    client ProviderHTTPClient
}

// Metadata returns the resource type name.
func (r *groupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
    resp.TypeName = req.ProviderTypeName + "_group"
}

// Schema defines the schema for the group resource.
func (r *groupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Required: true,
			},
                        "inventory_id": schema.Int64Attribute{
				Optional: true,
			},
			"name": schema.StringAttribute{
				Required: true,
			},
                        "description": schema.StringAttribute{
                                Optional: true,
                                Computed:    true,
                        },
                        "variables": schema.StringAttribute{
			        Computed:    true,
                                Optional: true,
                       },
		},
	}
}

// groupResourceModel maps the resource schema data.
type groupResourceModel struct {
	Id    types.Int64  `tfsdk:"id"`
        InventoryId   types.Int64  `tfsdk:"inventory_id"`
	Name          types.String `tfsdk:"name"`
	Description          types.String `tfsdk:"description"`
        Variables types.String `tfsdk:"variables"`
}

func IsValueProvided(value attr.Value) bool {
	return !value.IsNull() && !value.IsUnknown()
}

func (d *groupResourceModel) CreateRequestBody() (*bytes.Reader, error) {
	body := make(map[string]interface{})

	// Inventory id
        body["inventory"] = d.InventoryId.ValueInt64()

	// Name
	body["name"] = d.Name.ValueString()

        // Variables
        if IsValueProvided(d.Variables) {
		var variables map[string]interface{}
		_ = json.Unmarshal([]byte(d.Variables.ValueString()), &variables)
		body["variables"] = variables
	}

        // Description
	if IsValueProvided(d.Description) {
		body["description"] = d.Description
	}

	if len(body) == 0 {
		return nil, nil
	}
	json_raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(json_raw), nil
}

func (d *groupResourceModel) ParseHttpResponse(body []byte) error {
	/* Unmarshal the json string */
	var result map[string]interface{}
	err := json.Unmarshal(body, &result)
	if err != nil {
		return err
	}

	d.Name = types.StringValue(result["name"].(string))
	d.Description = types.StringValue(result["description"].(string))

	return nil
}

// Configure adds the provider configured client to the resource.
func (d *groupResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*AAPClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *AAPClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	d.client = client
}

func (r groupResource) CreateGroup(data GroupResourceModelInterface) error {

	req_data, err := data.CreateRequestBody()
	if err != nil {
		return err
	}

	var http_code int
	var body []byte
	post_url := "/api/v2/groups/"
	if req_data != nil {
		http_code, body, err = r.client.doRequest("POST", post_url, req_data)
	} else {
		http_code, body, err = r.client.doRequest("POST", post_url, nil)
	}

	if err != nil {
		return err
	}
	if http_code != http.StatusCreated {
		return fmt.Errorf("the server returned status code %d while attempting to create  a group with body %s req data %s data %s", http_code, body, req_data, data)
	}
	err = data.ParseHttpResponse(body)
	if err != nil {
		return fmt.Errorf("error while parsing the json response: " + err.Error())
	}
	return nil
}

func (r groupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data groupResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.CreateGroup(&data); err != nil {
		resp.Diagnostics.AddError(
			"Unexpected Resource Creation error",
			err.Error(),
		)
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r groupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
}

func (r groupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data groupResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)


	if err := r.CreateGroup(&data); err != nil {
		resp.Diagnostics.AddError(
			"Unexpected Resource Update error",
			err.Error(),
		)
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r groupResource) ReadGroup(data GroupResourceModelInterface) error {
	// Read existing Group
        httpCode, body, err := r.client.doRequest("GET", "/api/v2/groups/" , nil)
	if err != nil {
		return err
	}

	if httpCode != http.StatusOK {
		return fmt.Errorf("the server returned status code %d while attempting to Get groups", httpCode)
	}

	err = data.ParseHttpResponse(body)
	if err != nil {
		return err
	}
	return nil
}

func (r groupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data groupResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	err := r.ReadGroup(&data)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unexpected Resource Read error",
			err.Error(),
		)
		return
	}
	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
