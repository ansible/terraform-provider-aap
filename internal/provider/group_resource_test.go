package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
)

func TestGroupParseHttpResponse(t *testing.T) {
	t.Run("Basic Test", func(t *testing.T) {
		expected := GroupResourceModel{
			InventoryId: types.Int64Value(1),
			Name:        types.StringValue("group1"),
			Description: types.StringNull(),
			Variables:   jsontypes.NewNormalizedNull(),
			URL:         types.StringValue("/api/v2/groups/24/"),
			Id:          types.Int64Value(1),
		}
		g := GroupResourceModel{}
		body := []byte(`{"inventory": 1, "name": "group1", "url": "/api/v2/groups/24/", "id": 1, "description": "", "variables": ""}`)
		err := g.ParseHttpResponse(body)
		assert.NoError(t, err)
		if expected != g {
			t.Errorf("Expected (%s) not equal to actual (%s)", expected, g)
		}
	})
	t.Run("Test with variables", func(t *testing.T) {
		expected := GroupResourceModel{
			InventoryId: types.Int64Value(1),
			Name:        types.StringValue("group1"),
			URL:         types.StringValue("/api/v2/groups/24/"),
			Description: types.StringNull(),
			Variables:   jsontypes.NewNormalizedValue("{\"ansible_network_os\":\"ios\"}"),
			Id:          types.Int64Value(1),
		}
		g := GroupResourceModel{}
		body := []byte(`{"inventory": 1, "name": "group1", "url": "/api/v2/groups/24/", "variables": "{\"ansible_network_os\":\"ios\"}", "id": 1, "description": ""}`)
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

func TestGroupCreateRequestBody(t *testing.T) {
	t.Run("Basic Test", func(t *testing.T) {
		g := GroupResourceModel{
			InventoryId: types.Int64Value(1),
			Name:        types.StringValue("group1"),
			URL:         types.StringValue("/api/v2/groups/24/"),
		}
		body := []byte(`{"inventory": 1, "name": "group1"}`)
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
		body := []byte(`{"name": "group1", "inventory": 5,
		                 "description": "New Group",
						 "variables": "{\"ansible_network_os\":\"ios\"}"}`)

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
			Variables: jsontypes.NewNormalizedValue(
				"{\"ansible_network_os\":\"ios\",\"ansible_connection\":\"network_cli\",\"ansible_ssh_user\":\"ansible\",\"ansible_ssh_pass\":\"ansi\"}",
			),
			Description: types.StringValue("New Group"),
		}
		body := []byte(`{
    	"name": "group1",
    	"inventory": 5,
    	"description": "New Group",
    	"variables": "{\"ansible_network_os\":\"ios\",\"ansible_connection\":\"network_cli\",\"ansible_ssh_user\":\"ansible\",\"ansible_ssh_pass\":\"ansi\"}"
        }`)

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

// Acceptance tests

func getGroupResourceFromStateFile(s *terraform.State) (map[string]interface{}, error) {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "aap_group" {
			continue
		}
		groupURL := rs.Primary.Attributes["group_url"]
		body, err := testGetResource(groupURL)
		if err != nil {
			return nil, err
		}

		var result map[string]interface{}
		err = json.Unmarshal(body, &result)
		return result, err
	}
	return nil, fmt.Errorf("Group resource not found from state file")
}

func testAccCheckGroupExists(s *terraform.State) error {
	_, err := getGroupResourceFromStateFile(s)
	return err
}

func testAccCheckGroupUpdate(urlBefore *string, shouldDiffer bool) func(s *terraform.State) error {
	return func(s *terraform.State) error {
		var groupURL string
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "aap_group" {
				continue
			}
			groupURL = rs.Primary.Attributes["group_url"]
		}
		if len(groupURL) == 0 {
			return fmt.Errorf("Group resource not found from state file")
		}
		if len(*urlBefore) == 0 {
			*urlBefore = groupURL
			return nil
		}
		if groupURL == *urlBefore && shouldDiffer {
			return fmt.Errorf("Group resource URLs are equal while expecting them to differ. Before [%s] After [%s]", *urlBefore, groupURL)
		} else if groupURL != *urlBefore && !shouldDiffer {
			return fmt.Errorf("Group resource URLs differ while expecting them to be equals. Before [%s] After [%s]", *urlBefore, groupURL)
		}
		return nil
	}
}

func testAccGroupResourcePreCheck(t *testing.T) {
	// ensure provider requirements
	testAccPreCheck(t)

	requiredAAPGroupEnvVars := []string{
		"AAP_TEST_INVENTORY_ID",
	}

	for _, key := range requiredAAPGroupEnvVars {
		if v := os.Getenv(key); v == "" {
			t.Fatalf("'%s' environment variable must be set when running acceptance tests for group resource", key)
		}
	}
}

func TestAccAAPGroup_basic(t *testing.T) {
	var groupURLBefore string
	groupInventoryId := os.Getenv("AAP_TEST_INVENTORY_ID")
	randomName := acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)
	updatedName := "updated" + randomName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccGroupResourcePreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccBasicGroup(randomName, groupInventoryId),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("aap_group.test", "name", randomName),
					resource.TestCheckResourceAttr("aap_group.test", "inventory_id", groupInventoryId),
					resource.TestMatchResourceAttr("aap_group.test", "group_url", regexp.MustCompile("^/api/v2/groups/[0-9]*/$")),
					testAccCheckGroupExists,
				),
			},
			// Create and Read testing with same parameters
			{
				Config: testAccBasicGroup(randomName, groupInventoryId),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("aap_group.test", "name", randomName),
					resource.TestCheckResourceAttr("aap_group.test", "inventory_id", groupInventoryId),
					resource.TestMatchResourceAttr("aap_group.test", "group_url", regexp.MustCompile("^/api/v2/groups/[0-9]*/$")),
					testAccCheckGroupExists,
				),
			},
			// Update and Read testing
			{
				Config: testAccUpdateGroupComplete(updatedName, groupInventoryId),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("aap_group.test", "name", updatedName),
					resource.TestCheckResourceAttr("aap_group.test", "inventory_id", groupInventoryId),
					resource.TestMatchResourceAttr("aap_group.test", "group_url", regexp.MustCompile("^/api/v2/groups/[0-9]*/$")),
					testAccCheckGroupUpdate(&groupURLBefore, false),
				),
			},
		},
	})
}

func testAccBasicGroup(name, groupInventoryId string) string {
	return fmt.Sprintf(`
resource "aap_group" "test" {
  name = "%s"
  inventory_id = %s
}`, name, groupInventoryId)
}

func testAccUpdateGroupComplete(name, groupInventoryId string) string {
	return fmt.Sprintf(`
resource "aap_group" "test" {
  name = "%s"
  inventory_id = %s
  description = "A test group"
  variables = "{\"foo\": \"bar\"}"
}`, name, groupInventoryId)
}
