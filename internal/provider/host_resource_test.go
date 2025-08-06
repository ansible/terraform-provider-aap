package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/ansible/terraform-provider-aap/internal/provider/customtypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const hostVariable = "{\"foo\":\"bar\"}"

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
		{[]int64{3, 4, 5, 6, 7}, []int64{1, 2, 3, 4, 5}, []int64{6, 7}},
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
				Variables:   customtypes.NewAAPCustomStringUnknown(),
				Enabled:     basetypes.NewBoolValue(false),
				InventoryId: types.Int64Value(0),
			},
			expected: []byte(`{"inventory":0,"name":"test host","enabled":false}`),
		},
		{
			name: "test with null values",
			input: HostResourceModel{
				Name:        types.StringValue("test host"),
				Description: types.StringNull(),
				URL:         types.StringNull(),
				Variables:   customtypes.NewAAPCustomStringNull(),
				Enabled:     basetypes.NewBoolValue(false),
				InventoryId: types.Int64Value(0),
				Groups:      types.SetNull(types.Int64Type),
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
				Variables:   customtypes.NewAAPCustomStringValue(hostVariable),
			},
			expected: []byte(
				`{"inventory":1,"name":"host1","variables":"{\"foo\":\"bar\"}","enabled":false}`,
			),
		},
		{
			name: "test with all values",
			input: HostResourceModel{
				InventoryId: types.Int64Value(1),
				Name:        types.StringValue("host1"),
				Description: types.StringValue("A test host"),
				URL:         types.StringValue("/api/v2/hosts/1/"),
				Variables:   customtypes.NewAAPCustomStringValue("{\"foo\":\"bar\"}"),
				Enabled:     basetypes.NewBoolValue(false),
				Groups:      types.SetValueMust(types.Int64Type, []attr.Value{types.Int64Value(1), types.Int64Value(2)}),
			},
			expected: []byte(
				`{"inventory":1,"name":"host1","description":"A test host","variables":"{\"foo\":\"bar\"}","enabled":false}`,
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
			name:  "test with missing values",
			input: []byte(`{"inventory":1, "id": 0, "name": "host1", "url": "/api/v2/hosts/1/", "description": ""}`),
			expected: HostResourceModel{
				InventoryId: types.Int64Value(1),
				Id:          types.Int64Value(0),
				Name:        types.StringValue("host1"),
				URL:         types.StringValue("/api/v2/hosts/1/"),
				Description: types.StringNull(),
				Enabled:     basetypes.NewBoolValue(false),
			},
			errors: diag.Diagnostics{},
		},
		{
			name: "test with all values",
			input: []byte(`{"inventory":1,"description":"A basic test host","name":"host1","enabled":true,` +
				`"url":"/api/v2/hosts/1/","variables":"{\"foo\":\"bar\",\"nested\":{\"foobar\":\"baz\"}}",` +
				`"groups": [1, 2, 3], "id": 0}
				`),
			expected: HostResourceModel{
				InventoryId: types.Int64Value(1),
				Id:          types.Int64Value(0),
				Name:        types.StringValue("host1"),
				URL:         types.StringValue("/api/v2/hosts/1/"),
				Description: types.StringValue("A basic test host"),
				Variables:   customtypes.NewAAPCustomStringValue("{\"foo\":\"bar\",\"nested\":{\"foobar\":\"baz\"}}"),
				Enabled:     basetypes.NewBoolValue(true),
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

// Acceptance tests

func TestAccHostResource(t *testing.T) {
	var hostApiModel HostAPIModel
	inventoryName := acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)
	hostName := acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)
	groupName := acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)
	updatedName := "updated " + hostName
	updatedDescription := "A test host"
	updatedVariables := hostVariable

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Invalid variables testing
			{
				Config:      testAccHostResourceBadVariables(inventoryName, updatedName),
				ExpectError: regexp.MustCompile("Input type `str` is not a dictionary"),
			},
			// Create and Read testing
			{
				Config: testAccHostResourceMinimal(inventoryName, hostName),
				Check: resource.ComposeAggregateTestCheckFunc(
					checkBasicHostAttributes(t, resourceNameHost, hostName),
					testAccCheckHostResourceExists(resourceNameHost, &hostApiModel),
					testAccCheckHostResourceValues(&hostApiModel, hostName, "", ""),
				),
			},
			// Update and Read testing
			{
				Config: testAccHostResourceComplete(inventoryName, groupName, updatedName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckHostResourceExists(resourceNameHost, &hostApiModel),
					testAccCheckHostResourceValues(&hostApiModel, updatedName, updatedDescription, updatedVariables),
					checkBasicHostAttributes(t, resourceNameHost, updatedName),
					resource.TestCheckResourceAttr(resourceNameHost, "description", updatedDescription),
					resource.TestCheckResourceAttr(resourceNameHost, "variables", updatedVariables),
				),
			},
		},
		CheckDestroy: testAccCheckHostResourceDestroy,
	})
}

// testAccHostResourceMinimal returns a configuration for an AAP host with the required options only
func testAccHostResourceMinimal(inventoryName, hostName string) string {
	return fmt.Sprintf(`
resource "aap_inventory" "test" {
	name = "%s"
}

resource "aap_host" "test" {
  name = "%s"
  inventory_id = aap_inventory.test.id
}`, inventoryName, hostName)
}

// testAccHostResourceComplete returns a configuration for an AAP host with the provided name and all options
func testAccHostResourceComplete(inventoryName, groupName, hostName string) string {
	return fmt.Sprintf(`
resource "aap_inventory" "test" {
	name = "%s"
}

resource "aap_group" "test" {
	name = "%s"
	inventory_id = aap_inventory.test.id
}

resource "aap_host" "test" {
  name = "%s"
  inventory_id = aap_inventory.test.id
  description = "A test host"
  variables = "{\"foo\":\"bar\"}"
  enabled = true
  groups = [aap_group.test.id]
}`, inventoryName, groupName, hostName)
}

// testAccHostResourceBadVariables returns a configuration for an AAP host with the provided name and invalid variables.
func testAccHostResourceBadVariables(inventoryName, hostName string) string {
	return fmt.Sprintf(`
resource "aap_inventory" "test" {
  name = "%s"
}

resource "aap_host" "test" {
  name = "%s"
  inventory_id = aap_inventory.test.id
  variables = "Not valid JSON"
}`, inventoryName, hostName)
}

func TestAccHostResourceDeleteWithRetry(t *testing.T) {
	var hostApiModel HostAPIModel
	hostName := acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)
	jobTemplateID := os.Getenv("AAP_TEST_JOB_FOR_HOST_RETRY_ID") // ID of a Job Template that Sleeps for 15secs

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccHostResourceDeleteWithRetry(hostName, jobTemplateID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceNameHost, "enabled", "true"),
					resource.TestCheckResourceAttrSet(resourceNameHost, "id"),
					resource.TestMatchResourceAttr(resourceNameHost, "url", reHostURL),
					resource.TestCheckResourceAttr(resourceNameHost, "name", hostName),
					testAccCheckHostResourceExists(resourceNameHost, &hostApiModel),
					testAccCheckHostResourceValues(&hostApiModel, hostName, "", ""),
				),
			},
			// Delete Host Only
			{
				Config: testAccHostResourceDeleteWithRetry2(hostName),
			},
		},
		CheckDestroy: testAccCheckHostResourceDestroy,
	})
}

func testAccHostResourceDeleteWithRetry(hostName, jobTemplateID string) string {
	return fmt.Sprintf(`
resource "aap_host" "test" {
  name = "%s"
  inventory_id = 1
}

resource "aap_job" "test" {
  job_template_id = %s
  inventory_id    = 1
  extra_vars = "{\"sleep_interval\": \"5s\"}"
}`, hostName, jobTemplateID)
}

func testAccHostResourceDeleteWithRetry2(hostName string) string {
	return fmt.Sprintf(`
resource "aap_host" "test" {
  name = "%s"
  inventory_id = 1
}

removed {
  from = aap_job.test

  lifecycle {
    destroy = false
  }
}`, hostName)
}

// testAccCheckHostResourceExists queries the AAP API and retrieves the matching host.
func testAccCheckHostResourceExists(name string, hostApiModel *HostAPIModel) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		hostResource, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("host (%s) not found in state", name)
		}

		hostResponseBody, err := testGetResource(hostResource.Primary.Attributes["url"])
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

// testAccCheckHostResourcesValues verifies that the provided host retrieved from AAP contains the expected values.
func testAccCheckHostResourceValues(hostApiModel *HostAPIModel, name string, description string, variables string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
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
		if hostApiModel.Enabled != true {
			return fmt.Errorf("bad enabled value in AAP, expected %t, got: %t", true, hostApiModel.Enabled)
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
