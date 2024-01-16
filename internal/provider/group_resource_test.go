package provider

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/stretchr/testify/assert"
)

func TestParseHttpResponse(t *testing.T) {
	t.Run("Basic Test", func(t *testing.T) {
		expected := GroupResourceModel{
			Name:        types.StringValue("group1"),
			Description: types.StringValue(""),
			URL:         types.StringValue("/api/v2/groups/24/"),
		}
		g := GroupResourceModel{}
		body := []byte(`{"name": "group1", "url": "/api/v2/groups/24/", "description": ""}`)
		err := g.ParseHttpResponse(body)
		assert.NoError(t, err)
		if expected != g {
			t.Errorf("Expected (%s) not equal to actual (%s)", expected, g)
		}
	})
	t.Run("Test with variables", func(t *testing.T) {
		expected := GroupResourceModel{
			Name:        types.StringValue("group1"),
			URL:         types.StringValue("/api/v2/groups/24/"),
			Description: types.StringValue(""),
			Variables:   jsontypes.NewNormalizedValue("{\"ansible_network_os\":\"ios\"}"),
		}
		g := GroupResourceModel{}
		body := []byte(`{"name": "group1", "url": "/api/v2/groups/24/", "description": "", "variables": "{\"ansible_network_os\":\"ios\"}"}`)
		err := g.ParseHttpResponse(body)
		assert.NoError(t, err)
		if expected != g {
			t.Errorf("Expected (%s) not equal to actual (%s)", expected, g)
		}
	})
	t.Run("JSON error", func(t *testing.T) {
		g := GroupResourceModel{}
		body := []byte("Not valid JSON")
		err := g.ParseHttpResponse(body)
		assert.Error(t, err)

	})
}

func TestCreateRequestBody(t *testing.T) {
	t.Run("Basic Test", func(t *testing.T) {
		g := GroupResourceModel{
			InventoryId: basetypes.NewInt64Value(1),
			Name:        types.StringValue("group1"),
			URL:         types.StringValue("/api/v2/groups/24/"),
		}
		body := []byte(`{"inventory": 1, "name": "group1", "url": "/api/v2/groups/24/"}`)
		result, diags := g.CreateRequestBody()
		if diags.HasError() {
			t.Fatal(diags.Errors())
		}
		assert.JSONEq(t, string(body), string(result))
	})
	t.Run("Unknown Values", func(t *testing.T) {
		g := GroupResourceModel{
			InventoryId: basetypes.NewInt64Unknown(),
		}
		result, diags := g.CreateRequestBody()

		if diags.HasError() {
			t.Fatal(diags.Errors())
		}

		bytes.Equal(result, []byte(nil))

	})
	t.Run("All Values", func(t *testing.T) {
		g := GroupResourceModel{
			InventoryId: basetypes.NewInt64Value(5),
			Name:        types.StringValue("group1"),
			URL:         types.StringValue("/api/v2/groups/24/"),
			Variables:   jsontypes.NewNormalizedValue("{\"ansible_network_os\":\"ios\"}"),
			Description: types.StringValue("New Group"),
		}
		body := []byte(`{"name": "group1", "inventory": 5, "url": "/api/v2/groups/24/", "description": "New Group", "variables": "{\"ansible_network_os\":\"ios\"}"}`)

		result, diags := g.CreateRequestBody()
		if diags.HasError() {
			t.Fatal(diags.Errors())
		}
		assert.JSONEq(t, string(body), string(result))

	})
	t.Run("Multiple values for Variables", func(t *testing.T) {
		g := GroupResourceModel{
			InventoryId: basetypes.NewInt64Value(5),
			Name:        types.StringValue("group1"),
			URL:         types.StringValue("/api/v2/groups/24/"),
			Variables:   jsontypes.NewNormalizedValue("{\"ansible_network_os\":\"ios\", \"ansible_connection\":\"network_cli\", \"ansible_ssh_user\":\"ansible\", \"ansible_ssh_pass\":\"ansible\"}"),
			Description: types.StringValue("New Group"),
		}
		body := []byte(`{"name": "group1", "inventory": 5, "url": "/api/v2/groups/24/", "description": "New Group", "variables": "{\"ansible_network_os\":\"ios\", \"ansible_connection\":\"network_cli\", \"ansible_ssh_user\":\"ansible\", \"ansible_ssh_pass\":\"ansible\"}"}`)

		result, diags := g.CreateRequestBody()
		if diags.HasError() {
			t.Fatal(diags.Errors())
		}
		assert.JSONEq(t, string(body), string(result))

	})
}

type MockGroupResource struct {
	InventoryId string
	Name        string
	Description string
	URL         string
	Variables   string
	Response    map[string]string
}

func NewMockGroupResource(inventory, name, description, url, variables string) *MockGroupResource {
	return &MockGroupResource{
		InventoryId: inventory,
		URL:         url,
		Name:        name,
		Description: description,
		Variables:   variables,
		Response:    map[string]string{},
	}
}

func (d *MockGroupResource) GetURL() string {
	return d.URL
}

func (d *MockGroupResource) ParseHttpResponse(body []byte) error {
	err := json.Unmarshal(body, &d.Response)
	if err != nil {
		return err
	}
	return nil
}

func (d *MockGroupResource) CreateRequestBody() ([]byte, diag.Diagnostics) {
	var diags diag.Diagnostics

	m := make(map[string]interface{})
	m["Inventory"] = d.InventoryId
	m["Name"] = d.Name
	jsonRaw, err := json.Marshal(m)
	if err != nil {
		diags.AddError("Json Marshall Error", err.Error())
		return nil, diags
	}
	return jsonRaw, diags
}

var mResponse1 = map[string]string{
	"description": "",
	"inventory":   "1",
	"name":        "Group1",
}

var mResponse2 = map[string]string{
	"description": "Updated group",
	"inventory":   "1",
	"name":        "Group1",
}

var mResponse3 = map[string]string{
	"description": "",
	"inventory":   "3",
	"name":        "Group1",
	"variables":   "{\"ansible_network_os\": \"ios\"}",
}

func (c *MockHTTPClient) doRequest(method string, path string, data io.Reader) (*http.Response, []byte, error) {
	config := map[string]map[string]string{
		"/api/v2/groups/":   mResponse1,
		"/api/v2/groups/1/": mResponse2,
		"/api/v2/groups/2/": mResponse3,
	}

	if !slices.Contains(c.acceptMethods, method) {
		return nil, nil, nil
	}
	response, ok := config[path]
	if !ok {
		return &http.Response{StatusCode: http.StatusNotFound}, nil, nil
	}

	if data != nil {
		// add request info into response
		buf := new(strings.Builder)
		_, err := io.Copy(buf, data)
		if err != nil {
			return nil, nil, err
		}
		var mData map[string]string
		err = json.Unmarshal([]byte(buf.String()), &mData)
		if err != nil {
			return nil, nil, err
		}
		response = mergeStringMaps(response, mData)
	}
	result, err := json.Marshal(response)
	if err != nil {
		return nil, nil, err
	}
	return &http.Response{StatusCode: c.httpCode}, result, nil
}

func TestCreateGroup(t *testing.T) {
	t.Run("Create Group", func(t *testing.T) {
		g := NewMockGroupResource("1", "Group1", "", "", "")
		group := GroupResource{
			client: NewMockHTTPClient([]string{"POST", "post"}, http.StatusCreated),
		}

		diags := group.CreateGroup(g)
		if diags.HasError() {
			t.Errorf("Create Group failed")
			for _, d := range diags {
				t.Errorf("Summary = '%s' - details = '%s'", d.Summary(), d.Detail())
			}
		}
	})
}
func TestUpdateGroup(t *testing.T) {
	t.Run("Update Group", func(t *testing.T) {
		g := NewMockGroupResource("1", "Group1", "Updated Group", "/api/v2/groups/1/", "")
		group := GroupResource{
			client: NewMockHTTPClient([]string{"PUT", "put"}, http.StatusOK),
		}

		diags := group.UpdateGroup(g)
		if diags.HasError() {
			t.Errorf("Update Group failed")
			for _, d := range diags {
				t.Errorf("Summary = '%s' - details = '%s'", d.Summary(), d.Detail())
			}
		}
	})
	t.Run("Update Group with variables", func(t *testing.T) {
		g := NewMockGroupResource("2", "Group1", "Updated Group", "/api/v2/groups/2/", "{\"ansible_network_os\": \"ios\"}")
		group := GroupResource{
			client: NewMockHTTPClient([]string{"PUT", "put"}, http.StatusOK),
		}

		diags := group.UpdateGroup(g)
		if diags.HasError() {
			t.Errorf("Update Group with variables failed")
			for _, d := range diags {
				t.Errorf("Summary = '%s' - details = '%s'", d.Summary(), d.Detail())
			}
		}
	})
}
func TestReadGroup(t *testing.T) {
	t.Run("Read Group", func(t *testing.T) {
		g := NewMockGroupResource("1", "Group1", "", "/api/v2/groups/2/", "")
		group := GroupResource{
			client: NewMockHTTPClient([]string{"GET", "get"}, http.StatusOK),
		}

		diags := group.ReadGroup(g)
		if diags.HasError() {
			t.Errorf("Read Group failed")
			for _, d := range diags {
				t.Errorf("Summary = '%s' - details = '%s'", d.Summary(), d.Detail())
			}
		}
	})
	t.Run("Read Group with no URL", func(t *testing.T) {
		g := NewMockGroupResource("1", "Group1", "", "", "")
		group := GroupResource{
			client: NewMockHTTPClient([]string{"GET", "get"}, http.StatusOK),
		}

		err := group.ReadGroup(g)
		if err == nil {
			t.Errorf("Failure expected but the ReadJob did not fail!!")
		}

	})
}
