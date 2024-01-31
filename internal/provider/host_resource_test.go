package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func slicesEqual(slice1, slice2 []int64) bool {
	if len(slice1) != len(slice2) {
		return false
	}
	for i, v := range slice1 {
		if v != slice2[i] {
			return false
		}
	}
	return true
}

func TestSliceDifference(t *testing.T) {
	tests := []struct {
		slice1   []int64
		slice2   []int64
		expected []int64
	}{
		{[]int64{1, 2, 3, 4, 5}, []int64{3, 4, 5, 6, 7}, []int64{1, 2}},
		{[]int64{1, 2, 3}, []int64{3, 4, 5}, []int64{1, 2}},
		{[]int64{1, 2, 3}, []int64{}, []int64{1, 2, 3}},
		{[]int64{}, []int64{3, 4, 5}, []int64{}},
		{[]int64{1, 2, 3}, []int64{1, 2, 3}, []int64{}},
	}

	for _, test := range tests {
		result := sliceDifference(test.slice1, test.slice2)
		if !slicesEqual(result, test.expected) {
			t.Errorf("For %v and %v, expected %v, but got %v", test.slice1, test.slice2, test.expected, result)
		}
	}
}

func TestExtractIDs(t *testing.T) {
	tests := []struct {
		data     map[string]interface{}
		expected []int64
	}{
		{
			map[string]interface{}{
				"results": []interface{}{
					map[string]interface{}{"id": float64(1)},
					map[string]interface{}{"id": float64(2)},
					map[string]interface{}{"id": float64(3)},
				},
			},
			[]int64{1, 2, 3},
		},
		{
			map[string]interface{}{
				"results": []interface{}{
					map[string]interface{}{"id": float64(10)},
					map[string]interface{}{"id": float64(20)},
				},
			},
			[]int64{10, 20},
		},
		{
			map[string]interface{}{},
			[]int64{},
		},
	}

	for _, test := range tests {
		result := extractIDs(test.data)
		if !slicesEqual(result, test.expected) {
			t.Errorf("For %v, expected %v, but got %v", test.data, test.expected, result)
		}
	}
}

func TestHostResourceSchema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schemaRequest := fwresource.SchemaRequest{}
	schemaResponse := &fwresource.SchemaResponse{}

	// Instantiate the HostResource and call its Schema method
	NewHostResource().Schema(ctx, schemaRequest, schemaResponse)

	if schemaResponse.Diagnostics.HasError() {
		t.Fatalf("Schema method diagnostics: %+v", schemaResponse.Diagnostics)
	}

	// Validate the schema
	diagnostics := schemaResponse.Schema.ValidateImplementation(ctx)

	if diagnostics.HasError() {
		t.Fatalf("Schema validation diagnostics: %+v", diagnostics)
	}
}

func TestHostResourceCreateRequestBody(t *testing.T) {
	var testTable = []struct {
		name     string
		input    HostResourceModel
		expected []byte
	}{
		{
			name: "test with unknown values",
			input: HostResourceModel{
				Name:        types.StringValue("test host"),
				Description: types.StringUnknown(),
				URL:         types.StringUnknown(),
				Variables:   jsontypes.NewNormalizedUnknown(),
				Enabled:     basetypes.NewBoolValue(false),
				InventoryId: types.Int64Unknown(),
				InstanceId:  types.StringNull(),
			},
			expected: []byte(`{"inventory":0,"name":"test host","enabled":false}`),
		},
		{
			name: "test with null values",
			input: HostResourceModel{
				Name:        types.StringValue("test host"),
				Description: types.StringNull(),
				URL:         types.StringNull(),
				Variables:   jsontypes.NewNormalizedNull(),
				Enabled:     basetypes.NewBoolValue(false),
				InventoryId: types.Int64Null(),
				InstanceId:  types.StringNull(),
			},
			expected: []byte(`{"inventory":0,"name":"test host","enabled":false}`),
		},
		{
			name: "test with some values",
			input: HostResourceModel{
				InventoryId: types.Int64Value(1),
				Name:        types.StringValue("host1"),
				Description: types.StringNull(),
				URL:         types.StringValue("/api/v2/hosts/1/"),
				Variables:   jsontypes.NewNormalizedValue("{\"foo\":\"bar\"}"),
			},
			expected: []byte(
				`{"inventory":1,"name":"host1","variables":"{\"foo\":\"bar\"}","enabled":false}`,
			),
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			actual, diags := test.input.CreateRequestBody()
			if diags.HasError() {
				t.Fatal(diags.Errors())
			}
			if !bytes.Equal(test.expected, actual) {
				t.Errorf("Expected (%s) not equal to actual (%s)", test.expected, actual)
			}
		})
	}
}

func TestHostResourceParseHttpResponse(t *testing.T) {
	jsonError := diag.Diagnostics{}
	jsonError.AddError("Error parsing JSON response from AAP", "invalid character 'N' looking for beginning of value")

	var testTable = []struct {
		name     string
		input    []byte
		expected HostResourceModel
		errors   diag.Diagnostics
	}{
		{
			name:     "test with JSON error",
			input:    []byte("Not valid JSON"),
			expected: HostResourceModel{},
			errors:   jsonError,
		},
		{
			name: "test with missing values",
			input: []byte(`{"inventory":1,"name": "host1", "url": "/api/v2/hosts/1/", "description": "",` +
				` "variables": "{\"foo\":\"bar\",\"nested\":{\"foobar\":\"baz\"}}"}`),
			expected: HostResourceModel{
				InventoryId: types.Int64Value(1),
				Id:          types.Int64Value(0),
				Name:        types.StringValue("host1"),
				URL:         types.StringValue("/api/v2/hosts/1/"),
				Description: types.StringNull(),
				Variables:   jsontypes.NewNormalizedValue("{\"foo\":\"bar\",\"nested\":{\"foobar\":\"baz\"}}"),
				Enabled:     basetypes.NewBoolValue(false),
			},
			errors: diag.Diagnostics{},
		},
		{
			name: "test with all values",
			input: []byte(`{"inventory":1,"description":"A basic test host","name":"host1","enabled":false,` +
				`"url":"/api/v2/hosts/1/","variables":"{\"foo\":\"bar\",\"nested\":{\"foobar\":\"baz\"}}"}`),
			expected: HostResourceModel{
				InventoryId: types.Int64Value(1),
				Id:          types.Int64Value(0),
				Name:        types.StringValue("host1"),
				URL:         types.StringValue("/api/v2/hosts/1/"),
				Description: types.StringValue("A basic test host"),
				Variables:   jsontypes.NewNormalizedValue("{\"foo\":\"bar\",\"nested\":{\"foobar\":\"baz\"}}"),
				Enabled:     basetypes.NewBoolValue(false),
			},
			errors: diag.Diagnostics{},
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			resource := HostResourceModel{}
			diags := resource.ParseHttpResponse(test.input)
			if !test.errors.Equal(diags) {
				t.Errorf("Expected error diagnostics (%s), actual was (%s)", test.errors, diags)
			}
			if !reflect.DeepEqual(test.expected, resource) {
				t.Errorf("Expected (%s) not equal to actual (%s)", test.expected, resource)
			}
		})
	}
}

type MockHostResource struct {
	InventoryId string
	Id          string
	Name        string
	Description string
	URL         string
	Variables   string
	Response    map[string]string
}

func testAccHostResourcePreCheck(t *testing.T) {
	// ensure provider requirements
	testAccPreCheck(t)

	requiredAAPHostEnvVars := []string{
		"AAP_TEST_GROUP_ID",
		"AAP_TEST_INVENTORY_ID",
	}

	for _, key := range requiredAAPHostEnvVars {
		if v := os.Getenv(key); v == "" {
			t.Fatalf("'%s' environment variable must be set when running acceptance tests for job resource", key)
		}
	}
}

func TestAccHostResource(t *testing.T) {
	var hostApiModel HostAPIModel
	randomName := acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)
	updatedName := "updated " + randomName
	updatedDescription := "A test host"
	updatedVariables := "{\"foo\": \"bar\"}"

	groupId := os.Getenv("AAP_TEST_GROUP_ID")
	inventoryId := os.Getenv("AAP_TEST_INVENTORY_ID")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccHostResourcePreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Invalid variables testing
			{
				Config:      testAccHostResourceBadVariables(updatedName, inventoryId),
				ExpectError: regexp.MustCompile("A string value was provided that is not valid JSON string format"),
			},
			// Create and Read testing
			{
				Config: testAccHostResourceMinimal(randomName, inventoryId),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckHostResourceExists("aap_host.test", &hostApiModel),
					testAccCheckHostResourceValues(&hostApiModel, randomName, "", "", inventoryId),
					resource.TestCheckResourceAttr("aap_host.test", "name", randomName),
					resource.TestCheckResourceAttr("aap_host.test", "inventory_id", inventoryId),
					resource.TestCheckResourceAttr("aap_host.test", "enabled", "false"),
					resource.TestMatchResourceAttr("aap_host.test", "host_url", regexp.MustCompile("^/api/v2/hosts/[0-9]*/$")),
					resource.TestCheckResourceAttrSet("aap_host.test", "id"),
				),
			},
			// Update and Read testing
			{
				Config: testAccHostResourceComplete(updatedName, groupId, inventoryId),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckHostResourceExists("aap_host.test", &hostApiModel),
					testAccCheckHostResourceValues(&hostApiModel, updatedName, updatedDescription, updatedVariables, inventoryId),
					resource.TestCheckResourceAttr("aap_host.test", "name", updatedName),
					resource.TestCheckResourceAttr("aap_host.test", "inventory_id", inventoryId),
					resource.TestCheckResourceAttr("aap_host.test", "description", updatedDescription),
					resource.TestCheckResourceAttr("aap_host.test", "variables", updatedVariables),
					resource.TestCheckResourceAttr("aap_host.test", "enabled", "true"),
					resource.TestMatchResourceAttr("aap_host.test", "host_url", regexp.MustCompile("^/api/v2/hosts/[0-9]*/$")),
					resource.TestCheckResourceAttrSet("aap_host.test", "id"),
				),
			},
		},
		CheckDestroy: testAccCheckHostResourceDestroy,
	})
}

// testAccHostResourceMinimal returns a configuration for an AAP host with the required options only
func testAccHostResourceMinimal(name, inventoryId string) string {
	return fmt.Sprintf(`
resource "aap_host" "test" {
  name = "%s"
  inventory_id = %s
}`, name, inventoryId)
}

// testAccHostResourceComplete returns a configuration for an AAP host with the provided name and all options
func testAccHostResourceComplete(name, groupId, inventoryId string) string {
	return fmt.Sprintf(`
resource "aap_host" "test" {
  name = "%s"
  inventory_id = %s
  description = "A test host"
  variables = "{\"foo\": \"bar\"}"
  enabled = true
  groups = [%s]
}`, name, inventoryId, groupId)
}

// testAccHostResourceBadVariables returns a configuration for an AAP Inventory with the provided name and invalid variables.
func testAccHostResourceBadVariables(name, inventoryId string) string {
	return fmt.Sprintf(`
resource "aap_host" "test" {
  name = "%s"
  inventory_id = %s
  variables = "Not valid JSON"
}`, name, inventoryId)
}

// testAccCheckHostResourceExists queries the AAP API and retrieves the matching host.
func testAccCheckHostResourceExists(name string, hostApiModel *HostAPIModel) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		hostResource, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("host (%s) not found in state", name)
		}

		hostResponseBody, err := testGetResource(hostResource.Primary.Attributes["host_url"])
		if err != nil {
			return err
		}

		err = json.Unmarshal(hostResponseBody, &hostApiModel)
		if err != nil {
			return err
		}

		if hostApiModel.Id == 0 {
			return fmt.Errorf("host (%s) not found in AAP", hostResource.Primary.ID)
		}

		return nil
	}
}

// testAccCheckHostResourcesValues verifies that the provided inventory retrieved from AAP contains the expected values.
func testAccCheckHostResourceValues(hostApiModel *HostAPIModel, name string, description string, variables string, inventoryId string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		inv, err := strconv.ParseInt(inventoryId, 10, 64)
		if err != nil {
			return fmt.Errorf("could not convert \"%s\", to int64", inventoryId)
		}
		if hostApiModel.InventoryId != inv {
			return fmt.Errorf("bad host inventory id in AAP, expected %d, got: %d", inv, hostApiModel.InventoryId)
		}
		if hostApiModel.URL == "" {
			return fmt.Errorf("bad host URL in AAP, expected a URL path, got: %s", hostApiModel.URL)
		}
		if hostApiModel.Name != name {
			return fmt.Errorf("bad host name in AAP, expected \"%s\", got: %s", name, hostApiModel.Name)
		}
		if hostApiModel.Description != description {
			return fmt.Errorf("bad host description in AAP, expected \"%s\", got: %s", description, hostApiModel.Description)
		}
		if hostApiModel.Variables != variables {
			return fmt.Errorf("bad host variables in AAP, expected \"%s\", got: %s", variables, hostApiModel.Variables)
		}

		return nil
	}
}

// testAccCheckHostDestroy verifies the host has been destroyed.
func testAccCheckHostResourceDestroy(s *terraform.State) error {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "host" {
			continue
		}

		_, err := testGetResource(rs.Primary.Attributes["url"])
		if err == nil {
			return fmt.Errorf("host (%s) still exists.", rs.Primary.Attributes["id"])
		}

		if !strings.Contains(err.Error(), "404") {
			return err
		}
	}

	return nil
}
