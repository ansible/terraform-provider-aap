package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

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
	testTable := []struct {
		name     string
		input    HostResourceModel
		expected []byte
	}{
		{
			name: "test with unknown values",
			input: HostResourceModel{
				Name:                    types.StringValue("test host"),
				Description:             types.StringUnknown(),
				URL:                     types.StringUnknown(),
				Variables:               customtypes.NewAAPCustomStringUnknown(),
				Enabled:                 basetypes.NewBoolValue(false),
				InventoryId:             types.Int64Value(0),
				OperationTimeoutSeconds: types.Int64Unknown(),
			},
			expected: []byte(`{"inventory":0,"name":"test host","enabled":false}`),
		},
		{
			name: "test with null values",
			input: HostResourceModel{
				Name:                    types.StringValue("test host"),
				Description:             types.StringNull(),
				URL:                     types.StringNull(),
				Variables:               customtypes.NewAAPCustomStringNull(),
				Enabled:                 basetypes.NewBoolValue(false),
				InventoryId:             types.Int64Value(0),
				Groups:                  types.SetNull(types.Int64Type),
				OperationTimeoutSeconds: types.Int64Null(),
			},
			expected: []byte(`{"inventory":0,"name":"test host","enabled":false}`),
		},
		{
			name: "test with some values",
			input: HostResourceModel{
				InventoryId:             types.Int64Value(1),
				Name:                    types.StringValue("host1"),
				Description:             types.StringNull(),
				URL:                     types.StringValue("/api/v2/hosts/1/"),
				Variables:               customtypes.NewAAPCustomStringValue(hostVariable),
				OperationTimeoutSeconds: types.Int64Value(300),
			},
			expected: []byte(
				`{"inventory":1,"name":"host1","variables":"{\"foo\":\"bar\"}","enabled":false}`,
			),
		},
		{
			name: "test with all values",
			input: HostResourceModel{
				InventoryId:             types.Int64Value(1),
				Name:                    types.StringValue("host1"),
				Description:             types.StringValue("A test host"),
				URL:                     types.StringValue("/api/v2/hosts/1/"),
				Variables:               customtypes.NewAAPCustomStringValue("{\"foo\":\"bar\"}"),
				Enabled:                 basetypes.NewBoolValue(false),
				Groups:                  types.SetValueMust(types.Int64Type, []attr.Value{types.Int64Value(1), types.Int64Value(2)}),
				OperationTimeoutSeconds: types.Int64Value(600),
			},
			expected: []byte(
				`{"inventory":1,"name":"host1","description":"A test host","variables":"{\"foo\":\"bar\"}","enabled":false}`,
			),
		},
		{
			name: "test with custom timeout value",
			input: HostResourceModel{
				InventoryId:             types.Int64Value(1),
				Name:                    types.StringValue("host1"),
				Description:             types.StringValue("A test host with custom timeout"),
				URL:                     types.StringValue("/api/v2/hosts/1/"),
				Variables:               customtypes.NewAAPCustomStringValue("{\"timeout\":\"test\"}"),
				Enabled:                 basetypes.NewBoolValue(true),
				OperationTimeoutSeconds: types.Int64Value(120),
			},
			expected: []byte(
				`{"inventory":1,"name":"host1","description":"A test host with custom timeout","variables":"{\"timeout\":\"test\"}","enabled":true}`,
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

	testTable := []struct {
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

// Unit tests for createRetryStateChangeConf function

// Helper function to create test scenarios for successful operations
func createSuccessTestScenarios() []struct {
	name               string
	operationResponses []operationResponse
	timeout            time.Duration
	successStatusCodes []int
	operationName      string
	expectSuccess      bool
	expectedError      string
} {
	return []struct {
		name               string
		operationResponses []operationResponse
		timeout            time.Duration
		successStatusCodes []int
		operationName      string
		expectSuccess      bool
		expectedError      string
	}{
		{
			name: "successful operation - 200 OK",
			operationResponses: []operationResponse{
				{body: []byte("success"), diags: diag.Diagnostics{}, statusCode: 200},
			},
			timeout:            30 * time.Second,
			successStatusCodes: []int{200, 204},
			operationName:      "test operation",
			expectSuccess:      true,
		},
		{
			name: "successful operation - 204 No Content",
			operationResponses: []operationResponse{
				{body: []byte(""), diags: diag.Diagnostics{}, statusCode: 204},
			},
			timeout:            30 * time.Second,
			successStatusCodes: []int{200, 204},
			operationName:      "test operation",
			expectSuccess:      true,
		},
		{
			name: "successful operation - 202 Accepted (delete)",
			operationResponses: []operationResponse{
				{body: []byte(""), diags: diag.Diagnostics{}, statusCode: 202},
			},
			timeout:            30 * time.Second,
			successStatusCodes: []int{202, 204},
			operationName:      "delete operation",
			expectSuccess:      true,
		},
	}
}

// Helper function to create retry test scenarios
func createRetryTestScenarios() []struct {
	name               string
	operationResponses []operationResponse
	timeout            time.Duration
	successStatusCodes []int
	operationName      string
	expectSuccess      bool
	expectedError      string
} {
	return []struct {
		name               string
		operationResponses []operationResponse
		timeout            time.Duration
		successStatusCodes []int
		operationName      string
		expectSuccess      bool
		expectedError      string
	}{
		{
			name: "retry on 409 conflict then success",
			operationResponses: []operationResponse{
				{body: []byte(""), diags: diag.Diagnostics{}, statusCode: 409},
				{body: []byte("success"), diags: diag.Diagnostics{}, statusCode: 200},
			},
			timeout:            30 * time.Second,
			successStatusCodes: []int{200, 204},
			operationName:      "test operation",
			expectSuccess:      true,
		},
		{
			name: "retry on 429 rate limit then success",
			operationResponses: []operationResponse{
				{body: []byte(""), diags: diag.Diagnostics{}, statusCode: 429},
				{body: []byte("success"), diags: diag.Diagnostics{}, statusCode: 200},
			},
			timeout:            30 * time.Second,
			successStatusCodes: []int{200, 204},
			operationName:      "test operation",
			expectSuccess:      true,
		},
		{
			name: "retry on 500 internal error then success",
			operationResponses: []operationResponse{
				{body: []byte(""), diags: diag.Diagnostics{}, statusCode: 500},
				{body: []byte("success"), diags: diag.Diagnostics{}, statusCode: 200},
			},
			timeout:            30 * time.Second,
			successStatusCodes: []int{200, 204},
			operationName:      "test operation",
			expectSuccess:      true,
		},
		{
			name: "retry on 502 bad gateway then success",
			operationResponses: []operationResponse{
				{body: []byte(""), diags: diag.Diagnostics{}, statusCode: 502},
				{body: []byte("success"), diags: diag.Diagnostics{}, statusCode: 200},
			},
			timeout:            30 * time.Second,
			successStatusCodes: []int{200, 204},
			operationName:      "test operation",
			expectSuccess:      true,
		},
		{
			name: "retry on 503 service unavailable then success",
			operationResponses: []operationResponse{
				{body: []byte(""), diags: diag.Diagnostics{}, statusCode: 503},
				{body: []byte("success"), diags: diag.Diagnostics{}, statusCode: 200},
			},
			timeout:            30 * time.Second,
			successStatusCodes: []int{200, 204},
			operationName:      "test operation",
			expectSuccess:      true,
		},
		{
			name: "retry on 504 gateway timeout then success",
			operationResponses: []operationResponse{
				{body: []byte(""), diags: diag.Diagnostics{}, statusCode: 504},
				{body: []byte("success"), diags: diag.Diagnostics{}, statusCode: 200},
			},
			timeout:            30 * time.Second,
			successStatusCodes: []int{200, 204},
			operationName:      "test operation",
			expectSuccess:      true,
		},
		{
			name: "retry on 408 request timeout then success",
			operationResponses: []operationResponse{
				{body: []byte(""), diags: diag.Diagnostics{}, statusCode: 408},
				{body: []byte("success"), diags: diag.Diagnostics{}, statusCode: 200},
			},
			timeout:            30 * time.Second,
			successStatusCodes: []int{200, 204},
			operationName:      "test operation",
			expectSuccess:      true,
		},
	}
}

// Helper function to create error test scenarios
func createErrorTestScenarios() []struct {
	name               string
	operationResponses []operationResponse
	timeout            time.Duration
	successStatusCodes []int
	operationName      string
	expectSuccess      bool
	expectedError      string
} {
	return []struct {
		name               string
		operationResponses []operationResponse
		timeout            time.Duration
		successStatusCodes []int
		operationName      string
		expectSuccess      bool
		expectedError      string
	}{
		{
			name: "non-retryable error - 400 bad request",
			operationResponses: []operationResponse{
				{body: []byte("bad request"), diags: diag.Diagnostics{}, statusCode: 400},
			},
			timeout:            30 * time.Second,
			successStatusCodes: []int{200, 204},
			operationName:      "test operation",
			expectSuccess:      false,
			expectedError:      "non-retryable HTTP status 400",
		},
		{
			name: "non-retryable error - 404 not found",
			operationResponses: []operationResponse{
				{body: []byte("not found"), diags: diag.Diagnostics{}, statusCode: 404},
			},
			timeout:            30 * time.Second,
			successStatusCodes: []int{200, 204},
			operationName:      "test operation",
			expectSuccess:      false,
			expectedError:      "non-retryable HTTP status 404",
		},
		{
			name: "success status but with diagnostics errors",
			operationResponses: []operationResponse{
				{
					body: []byte("success"),
					diags: func() diag.Diagnostics {
						d := diag.Diagnostics{}
						d.AddError("Test Error", "Something went wrong")
						return d
					}(),
					statusCode: 200,
				},
			},
			timeout:            30 * time.Second,
			successStatusCodes: []int{200, 204},
			operationName:      "diagnostics error operation",
			expectSuccess:      false,
			expectedError:      "diagnostics error operation succeeded but diagnostics has errors",
		},
	}
}

// Helper function to run a single test scenario
func runRetryTestScenario(t *testing.T, test struct {
	name               string
	operationResponses []operationResponse
	timeout            time.Duration
	successStatusCodes []int
	operationName      string
	expectSuccess      bool
	expectedError      string
},
) {
	t.Run(test.name, func(t *testing.T) {
		// Create mock operation that cycles through responses
		callCount := 0
		mockOperation := func() ([]byte, diag.Diagnostics, int) {
			if callCount < len(test.operationResponses) {
				response := test.operationResponses[callCount]
				callCount++
				return response.body, response.diags, response.statusCode
			}
			// If we've run out of responses, return the last one
			response := test.operationResponses[len(test.operationResponses)-1]
			return response.body, response.diags, response.statusCode
		}

		// Create the StateChangeConf
		stateConf := createRetryStateChangeConf(
			mockOperation,
			test.timeout,
			test.successStatusCodes,
			test.operationName,
		)

		// Test the refresh function - call it as many times as we have responses
		var finalResult interface{}
		var finalState string
		var finalErr error

		for i := 0; i < len(test.operationResponses); i++ {
			finalResult, finalState, finalErr = stateConf.Refresh()

			// For retry scenarios, if we're not on the last call and state is "retrying", continue
			if i < len(test.operationResponses)-1 && finalState == "retrying" {
				continue
			}
			// Otherwise break (success or error on final attempt)
			break
		}

		if test.expectSuccess {
			if finalErr != nil {
				t.Errorf("Expected success, but got error: %v", finalErr)
			}
			if finalState != "success" {
				t.Errorf("Expected state 'success', got %s", finalState)
			}
			if finalResult == nil {
				t.Error("Expected non-nil result for successful operation")
			}
		} else {
			if finalErr == nil {
				t.Error("Expected error, but got nil")
			}
			if test.expectedError != "" && !strings.Contains(finalErr.Error(), test.expectedError) {
				t.Errorf("Expected error to contain '%s', got: %v", test.expectedError, finalErr)
			}
		}
	})
}

func TestCreateRetryStateChangeConf(t *testing.T) {
	// Test successful operations
	successTests := createSuccessTestScenarios()
	for _, test := range successTests {
		runRetryTestScenario(t, test)
	}

	// Test retry scenarios
	retryTests := createRetryTestScenarios()
	for _, test := range retryTests {
		runRetryTestScenario(t, test)
	}

	// Test error scenarios
	errorTests := createErrorTestScenarios()
	for _, test := range errorTests {
		runRetryTestScenario(t, test)
	}
}

// TestCreateRetryStateChangeConfTimeoutConfiguration verifies that the retry timing configuration
// is set up correctly based on the total timeout duration, particularly testing the jitter behavior.
//
// This test validates the anti-thundering-herd mechanism where:
// 1. Short/medium timeouts (≤30s): Use fixed 2-second MinTimeout (no jitter needed for short operations)
// 2. Long timeouts (>30s): Add crypto-secure jitter (0-2s) to MinTimeout to spread out retry attempts
//
// The jitter prevents multiple clients from retrying simultaneously, which could overwhelm the API.
// This is especially important for long-running operations where many terraform processes might
// be waiting for the same resource to become available.
//
// Test scenarios:
// - 10s timeout: MinTimeout = 2s (no jitter)
// - 30s timeout: MinTimeout = 2s (no jitter)
// - 60s timeout: MinTimeout = 2-4s (with jitter)
//
// Also verifies that Delay is consistently set to 1 second (initial delay before first retry).
func TestCreateRetryStateChangeConfTimeoutConfiguration(t *testing.T) {
	operation := func() ([]byte, diag.Diagnostics, int) {
		return []byte("success"), diag.Diagnostics{}, 200
	}

	tests := []struct {
		name                    string
		timeout                 time.Duration
		expectedMinTimeoutRange []time.Duration // [min, max] range for jitter
	}{
		{
			name:                    "short timeout - no jitter",
			timeout:                 10 * time.Second,
			expectedMinTimeoutRange: []time.Duration{2 * time.Second, 2 * time.Second},
		},
		{
			name:                    "medium timeout - no jitter",
			timeout:                 30 * time.Second,
			expectedMinTimeoutRange: []time.Duration{2 * time.Second, 2 * time.Second},
		},
		{
			name:                    "long timeout - with jitter",
			timeout:                 60 * time.Second,
			expectedMinTimeoutRange: []time.Duration{2 * time.Second, 4 * time.Second},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stateConf := createRetryStateChangeConf(operation, test.timeout, []int{200}, "test")

			// Verify basic timing configuration
			if stateConf.Delay != 1*time.Second {
				t.Errorf("Expected Delay 1s, got %v", stateConf.Delay)
			}

			// Verify MinTimeout is within expected range
			minTimeout := stateConf.MinTimeout
			if minTimeout < test.expectedMinTimeoutRange[0] || minTimeout > test.expectedMinTimeoutRange[1] {
				t.Errorf("Expected MinTimeout between %v and %v, got %v",
					test.expectedMinTimeoutRange[0], test.expectedMinTimeoutRange[1], minTimeout)
			}
		})
	}
}

// TestCreateRetryStateChangeConfAllRetryableStatusCodes verifies that each individual HTTP status code
// we consider "retryable" properly triggers retry behavior instead of failing immediately.
//
// This test is critical because it:
// 1. Validates each retryable status code in isolation (409, 408, 429, 500, 502, 503, 504)
// 2. Ensures the state machine transitions correctly: first call returns "retrying", second call succeeds
// 3. Protects against regressions where someone accidentally removes a status code from retry logic
// 4. Documents all retryable status codes in one place for maintainability
//
// Test pattern for each status code:
// - First call: Returns the retryable status → Should get "retrying" state (no error)
// - Second call: Returns HTTP 200 success → Should get "success" state with result
func TestCreateRetryStateChangeConfAllRetryableStatusCodes(t *testing.T) {
	retryableStatusCodes := []int{409, 408, 429, 500, 502, 503, 504}

	for _, statusCode := range retryableStatusCodes {
		t.Run(fmt.Sprintf("retryable_status_%d", statusCode), func(t *testing.T) {
			callCount := 0
			operation := func() ([]byte, diag.Diagnostics, int) {
				callCount++
				if callCount == 1 {
					return []byte(""), diag.Diagnostics{}, statusCode
				}
				return []byte("success"), diag.Diagnostics{}, 200
			}

			stateConf := createRetryStateChangeConf(operation, 30*time.Second, []int{200}, "test")

			// First call should return "retrying" state
			result, state, err := stateConf.Refresh()
			if err != nil {
				t.Errorf("First call should not error for retryable status %d, got: %v", statusCode, err)
			}
			if state != "retrying" {
				t.Errorf("Expected state 'retrying' for status %d, got %s", statusCode, state)
			}
			if result != nil {
				t.Errorf("Expected nil result for retrying state, got %v", result)
			}

			// Second call should succeed
			result, state, err = stateConf.Refresh()
			if err != nil {
				t.Errorf("Second call should succeed, got error: %v", err)
			}
			if state != "success" {
				t.Errorf("Expected state 'success', got %s", state)
			}
			if result == nil {
				t.Error("Expected non-nil result for successful operation")
			}
		})
	}
}

// Helper struct for test data
type operationResponse struct {
	body       []byte
	diags      diag.Diagnostics
	statusCode int
}
