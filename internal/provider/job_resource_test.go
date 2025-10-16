package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ansible/terraform-provider-aap/internal/provider/customtypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflogtest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const (
	statusRunningConst = "running"
	statusPendingConst = "pending"
)

func TestJobResourceSchema(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	schemaRequest := fwresource.SchemaRequest{}
	schemaResponse := &fwresource.SchemaResponse{}

	// Instantiate the JobResource and call its Schema method
	NewJobResource().Schema(ctx, schemaRequest, schemaResponse)

	if schemaResponse.Diagnostics.HasError() {
		t.Fatalf("Schema method diagnostics: %+v", schemaResponse.Diagnostics)
	}

	// Validate the schema
	diagnostics := schemaResponse.Schema.ValidateImplementation(ctx)

	if diagnostics.HasError() {
		t.Fatalf("Schema validation diagnostics: %+v", diagnostics)
	}
}

func TestIsFinalStateAAPJob(t *testing.T) {
	testTable := []struct {
		name     string
		input    string
		expected bool
	}{
		{name: "state new", input: "new", expected: false},
		{name: "state pending", input: "pending", expected: false},
		{name: "state waiting", input: "waiting", expected: false},
		{name: "state running", input: "running", expected: false},
		{name: "state successful", input: "successful", expected: true},
		{name: "state failed", input: "failed", expected: true},
		{name: "state error", input: "error", expected: true},
		{name: "state canceled", input: "canceled", expected: true},
		{name: "state empty string", input: "", expected: false},
		{name: "state random string", input: "random", expected: false},
	}
	for _, tc := range testTable {
		t.Run(tc.name, func(t *testing.T) {
			result := IsFinalStateAAPJob(tc.input)
			if result != tc.expected {
				t.Errorf("expected %t, got result %t", tc.expected, result)
			}
		})
	}
}

func TestJobResourceCreateRequestBody(t *testing.T) {
	testTable := []struct {
		name     string
		input    JobResourceModel
		expected []byte
	}{
		{
			name: "unknown values",
			input: JobResourceModel{JobModel: JobModel{
				ExtraVars:   customtypes.NewAAPCustomStringUnknown(),
				InventoryID: basetypes.NewInt64Unknown(),
				TemplateID:  types.Int64Value(1),
			}},
			expected: []byte(`{}`),
		},
		{
			name: "null values",
			input: JobResourceModel{JobModel: JobModel{
				ExtraVars:   customtypes.NewAAPCustomStringNull(),
				InventoryID: basetypes.NewInt64Null(),
				TemplateID:  types.Int64Value(1),
			}},
			expected: []byte(`{}`),
		},
		{
			name: "extra vars only",
			input: JobResourceModel{JobModel: JobModel{
				ExtraVars:   customtypes.NewAAPCustomStringValue("{\"test_name\":\"extra_vars\", \"provider\":\"aap\"}"),
				InventoryID: basetypes.NewInt64Null(),
			}},
			expected: []byte(`{"extra_vars":"{\"test_name\":\"extra_vars\", \"provider\":\"aap\"}"}`),
		},
		{
			name: "inventory vars only",
			input: JobResourceModel{JobModel: JobModel{
				ExtraVars:   customtypes.NewAAPCustomStringNull(),
				InventoryID: basetypes.NewInt64Value(201),
			}},
			expected: []byte(`{"inventory": 201}`),
		},
		{
			name: "combined",
			input: JobResourceModel{JobModel: JobModel{
				ExtraVars:   customtypes.NewAAPCustomStringValue("{\"test_name\":\"extra_vars\", \"provider\":\"aap\"}"),
				InventoryID: basetypes.NewInt64Value(3),
			}},
			expected: []byte(`{"inventory":3,"extra_vars":"{\"test_name\":\"extra_vars\", \"provider\":\"aap\"}"}`),
		},
		{
			name: "manual_triggers",
			input: JobResourceModel{JobModel: JobModel{
				InventoryID: basetypes.NewInt64Value(3),
			},
				Triggers: types.MapNull(types.StringType),
			},
			expected: []byte(`{"inventory": 3}`),
		},
		{
			name: "wait_for_completed parameters",
			input: JobResourceModel{JobModel: JobModel{
				InventoryID:              basetypes.NewInt64Value(3),
				TemplateID:               types.Int64Value(1),
				WaitForCompletion:        basetypes.NewBoolValue(true),
				WaitForCompletionTimeout: basetypes.NewInt64Value(60),
			}},
			expected: []byte(`{"inventory":3}`),
		},
	}

	for _, tc := range testTable {
		t.Run(tc.name, func(t *testing.T) {
			computed, diags := tc.input.CreateRequestBody()
			if diags.HasError() {
				t.Fatal(diags.Errors())
			}
			if tc.expected == nil || computed == nil {
				if tc.expected == nil && computed != nil {
					t.Fatal("expected nil but result is not nil", string(computed))
				}
				if tc.expected != nil && computed == nil {
					t.Fatal("expected result not nil but result is nil", string(computed))
				}
			} else {
				test, err := DeepEqualJSONByte(tc.expected, computed)
				if err != nil {
					t.Errorf("expected (%s)", string(tc.expected))
					t.Errorf("computed (%s)", string(computed))
					t.Fatal("Error while comparing results " + err.Error())
				}
				if !test {
					t.Errorf("expected (%s)", string(tc.expected))
					t.Errorf("computed (%s)", string(computed))
				}
			}
		})
	}
}

func TestJobResourceParseHttpResponse(t *testing.T) {
	templateID := basetypes.NewInt64Value(1)
	inventoryID := basetypes.NewInt64Value(2)
	extraVars := customtypes.NewAAPCustomStringNull()
	jsonError := diag.Diagnostics{}
	jsonError.AddError("Error parsing JSON response from AAP", "invalid character 'N' looking for beginning of value")

	testTable := []struct {
		name     string
		input    []byte
		expected JobResourceModel
		errors   diag.Diagnostics
	}{
		{
			name:     "JSON error",
			input:    []byte("Not valid JSON"),
			expected: JobResourceModel{},
			errors:   jsonError,
		},
		{
			name:  "no ignored fields",
			input: []byte(`{"inventory":2,"job_template":1,"job_type": "run", "url": "/api/v2/jobs/14/", "status": "pending"}`),
			expected: JobResourceModel{JobModel: JobModel{
				TemplateID:  templateID,
				InventoryID: inventoryID,
				ExtraVars:   extraVars,
			},
				Type:          types.StringValue("run"),
				URL:           types.StringValue("/api/v2/jobs/14/"),
				Status:        types.StringValue("pending"),
				IgnoredFields: types.ListNull(types.StringType),
			},
			errors: diag.Diagnostics{},
		},
		{
			name: "ignored fields",
			input: []byte(`{"inventory":2,"job_template":1,"job_type": "run", "url": "/api/v2/jobs/14/", "status":
			"pending", "ignored_fields": {"extra_vars": "{\"bucket_state\":\"absent\"}"}}`),
			expected: JobResourceModel{JobModel: JobModel{
				TemplateID:  templateID,
				InventoryID: inventoryID,
				ExtraVars:   extraVars,
			},
				Type:          types.StringValue("run"),
				URL:           types.StringValue("/api/v2/jobs/14/"),
				Status:        types.StringValue("pending"),
				IgnoredFields: basetypes.NewListValueMust(types.StringType, []attr.Value{types.StringValue("extra_vars")}),
			},
			errors: diag.Diagnostics{},
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			resource := JobResourceModel{}
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

func getJobResourceFromStateFile(s *terraform.State) (map[string]interface{}, error) {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "aap_job" {
			continue
		}
		jobURL := rs.Primary.Attributes["url"]
		body, err := testGetResource(jobURL)
		if err != nil {
			return nil, err
		}

		var result map[string]interface{}
		err = json.Unmarshal(body, &result)
		return result, err
	}
	return nil, fmt.Errorf("Job resource not found from state file")
}

func testAccCheckJobExists(s *terraform.State) error {
	_, err := getJobResourceFromStateFile(s)
	return err
}

func testAccCheckJobUpdate(urlBefore *string, shouldDiffer bool) func(s *terraform.State) error {
	return func(s *terraform.State) error {
		var jobURL string
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "aap_job" {
				continue
			}
			jobURL = rs.Primary.Attributes["url"]
		}
		if len(jobURL) == 0 {
			return fmt.Errorf("Job resource not found from state file")
		}
		if len(*urlBefore) == 0 {
			*urlBefore = jobURL
			return nil
		}
		if jobURL == *urlBefore && shouldDiffer {
			return fmt.Errorf("Job resource URLs are equal while expecting them to differ. Before [%s] After [%s]", *urlBefore, jobURL)
		} else if jobURL != *urlBefore && !shouldDiffer {
			return fmt.Errorf("Job resource URLs differ while expecting them to be equals. Before [%s] After [%s]", *urlBefore, jobURL)
		}
		return nil
	}
}

func testAccJobResourcePreCheck(t *testing.T) {
	// ensure provider requirements
	testAccPreCheck(t)

	requiredAAPJobEnvVars := []string{
		"AAP_TEST_JOB_TEMPLATE_ID",
		"AAP_TEST_JOB_FOR_HOST_RETRY_ID",
	}

	for _, key := range requiredAAPJobEnvVars {
		if v := os.Getenv(key); v == "" {
			t.Fatalf("'%s' environment variable must be set when running acceptance tests for job resource", key)
		}
	}
}

func TestAccAAPJob_basic(t *testing.T) {
	jobTemplateID := os.Getenv("AAP_TEST_JOB_TEMPLATE_ID")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccJobResourcePreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccBasicJob(jobTemplateID),
				Check: resource.ComposeAggregateTestCheckFunc(
					checkBasicJobAttributes(t, resourceNameJob, reJobStatus),
					testAccCheckJobExists,
				),
			},
		},
	})
}

func TestAccAAPJob_UpdateWithSameParameters(t *testing.T) {
	var jobURLBefore string

	jobTemplateID := os.Getenv("AAP_TEST_JOB_TEMPLATE_ID")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccJobResourcePreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccBasicJob(jobTemplateID),
				Check: resource.ComposeAggregateTestCheckFunc(
					checkBasicJobAttributes(t, resourceNameJob, reJobStatus),
					testAccCheckJobUpdate(&jobURLBefore, false),
				),
			},
			{
				Config: testAccBasicJob(jobTemplateID),
				Check: resource.ComposeAggregateTestCheckFunc(
					checkBasicJobAttributes(t, resourceNameJob, reJobStatus),
					testAccCheckJobUpdate(&jobURLBefore, false),
				),
			},
		},
	})
}

func TestAccAAPJob_UpdateWithNewInventoryIdPromptOnLaunch(t *testing.T) {
	// In order to run the this test for the job resource, you must have a working job template already in your AAP instance.
	// The job template used must be set to require an inventory on launch. Export the id of this job template into the
	// environment variable AAP_TEST_JOB_TEMPLATE_ID. Otherwise this test will fail when running the suite.

	var jobURLBefore string

	inventoryName := acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)
	jobTemplateID := os.Getenv("AAP_TEST_JOB_TEMPLATE_ID")
	ctx := t.Context()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccJobResourcePreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccBasicJob(jobTemplateID),
				Check: resource.ComposeAggregateTestCheckFunc(
					checkBasicJobAttributes(t, resourceNameJob, reJobStatus),
					testAccCheckJobUpdate(&jobURLBefore, false),
				),
			},
			{
				Config: testAccUpdateJobWithInventoryID(inventoryName, jobTemplateID),
				Check: resource.ComposeAggregateTestCheckFunc(
					checkBasicJobAttributes(t, resourceNameJob, reJobStatus),
					testAccCheckJobUpdate(&jobURLBefore, true),
					// Wait for the job to finish so the inventory can be deleted
					testAccCheckJobPause(ctx, resourceNameJob),
				),
			},
		},
	})
}

func TestAccAAPJob_UpdateWithTrigger(t *testing.T) {
	var jobURLBefore string

	jobTemplateID := os.Getenv("AAP_TEST_JOB_TEMPLATE_ID")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccJobResourcePreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccBasicJob(jobTemplateID),
				Check: resource.ComposeAggregateTestCheckFunc(
					checkBasicJobAttributes(t, resourceNameJob, reJobStatus),
					testAccCheckJobUpdate(&jobURLBefore, false),
				),
			},
			{
				Config: testAccUpdateJobWithTrigger(jobTemplateID),
				Check: resource.ComposeAggregateTestCheckFunc(
					checkBasicJobAttributes(t, resourceNameJob, reJobStatus),
					testAccCheckJobUpdate(&jobURLBefore, true),
				),
			},
		},
	})
}

// TestAccAAPJob_WaitForCompletion tests that job status is correctly updated to final state
// when wait_for_completion=true. This test demonstrates the bug described in Issue #78
// Expected to FAIL on main branch, PASS after PR #131 and #132 are merged.
func TestAccAAPJob_WaitForCompletion(t *testing.T) {
	jobTemplateID := os.Getenv("AAP_TEST_JOB_FOR_HOST_RETRY_ID")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccJobResourcePreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccJobWithWaitForCompletion(jobTemplateID),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckJobExists,
					// This check should FAIL on main branch due to issue #78
					// The job status should be "successful" or "failed", not "pending"
					resource.TestCheckResourceAttrWith("aap_job.test", "status", func(value string) error {
						if value == statusPendingConst {
							return fmt.Errorf("issue #78 bug: job status is still in 'pending' instead of final state")
						}
						if !IsFinalStateAAPJob(value) {
							return fmt.Errorf("job status '%s' is not a final state", value)
						}
						return nil
					}),
					// Verify wait_for_completion was actually used
					resource.TestCheckResourceAttr("aap_job.test", "wait_for_completion", "true"),
					resource.TestCheckResourceAttr("aap_job.test", "wait_for_completion_timeout_seconds", "300"),
				),
			},
		},
	})
}

// testAccCheckJobPause is designed to force the acceptance test framework to wait
// until a job is finished. This is needed when the associated inventory also must be
// deleted.
func testAccCheckJobPause(ctx context.Context, name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		var jobApiModel JobAPIModel
		job, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("job (%s) not found in terraform state", name)
		}

		timeout := 240 * time.Second
		err := retry.RetryContext(ctx, timeout, func() *retry.RetryError {
			body, err := testGetResource(job.Primary.Attributes["url"])
			if err != nil {
				return retry.NonRetryableError(err)
			}
			err = json.Unmarshal(body, &jobApiModel)
			if err != nil {
				return retry.NonRetryableError(err)
			}
			if IsFinalStateAAPJob(jobApiModel.Status) {
				return nil
			}
			return retry.RetryableError(fmt.Errorf("error when waiting for AAP job to complete in test"))
		})
		if err != nil {
			return err
		}

		return nil
	}
}

func testAccBasicJob(jobTemplateID string) string {
	return fmt.Sprintf(`
resource "aap_job" "test" {
	job_template_id   = %s
}
`, jobTemplateID)
}

func testAccUpdateJobWithInventoryID(inventoryName, jobTemplateID string) string {
	return fmt.Sprintf(`
resource "aap_inventory" "test" {
  name = "%s"
}

resource "aap_job" "test" {
	job_template_id   = %s
	inventory_id = aap_inventory.test.id
}
`, inventoryName, jobTemplateID)
}

func testAccUpdateJobWithTrigger(jobTemplateID string) string {
	return fmt.Sprintf(`
resource "aap_job" "test" {
	job_template_id   = %s
	triggers = {
		"key1" = "value1"
		"key2" = "value2"
	}
}
`, jobTemplateID)
}

func testAccJobWithWaitForCompletion(jobTemplateID string) string {
	return fmt.Sprintf(`
resource "aap_job" "test" {
	job_template_id                     = %s
	wait_for_completion                 = true
	wait_for_completion_timeout_seconds = 300
}
`, jobTemplateID)
}

func TestAccAAPJob_disappears(t *testing.T) {
	var jobUrl string

	jobTemplateID := os.Getenv("AAP_TEST_JOB_TEMPLATE_ID")
	ctx := t.Context()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccJobResourcePreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Apply a basic terraform plan that creates an AAP Job and records it to state with a URL.
			{
				Config: testAccBasicJob(jobTemplateID),
				Check: resource.ComposeAggregateTestCheckFunc(
					checkBasicJobAttributes(t, resourceNameJob, reJobStatus),
					testAccCheckJobUpdate(&jobUrl, false),
				),
			},
			// Wait for the job to finish.
			{
				Config: testAccBasicJob(jobTemplateID),
				Check: resource.ComposeAggregateTestCheckFunc(
					checkBasicJobAttributes(t, resourceNameJob, reJobStatus),
					// Wait for the job to finish so the inventory can be deleted
					testAccCheckJobPause(ctx, resourceNameJob),
				),
			},
			// Confirm the job is finished (fewer options in status), then delete directly via API, outside of terraform.
			{
				Config: testAccBasicJob(jobTemplateID),
				Check: resource.ComposeAggregateTestCheckFunc(
					checkBasicJobAttributes(t, resourceNameJob, reJobStatusFinal),
					testAccDeleteJob(&jobUrl),
				),
				ExpectNonEmptyPlan: true,
			},
			// Apply the plan again and confirm the job is re-created with a different URL.
			{
				Config: testAccBasicJob(jobTemplateID),
				Check: resource.ComposeAggregateTestCheckFunc(
					checkBasicJobAttributes(t, resourceNameJob, reJobStatus),
					testAccCheckJobUpdate(&jobUrl, true),
				),
			},
		},
	})
}

func testAccDeleteJob(jobUrl *string) func(s *terraform.State) error {
	return func(_ *terraform.State) error {
		_, err := testDeleteResource(*jobUrl)
		return err
	}
}

// TestRetryUntilAAPJobReachesAnyFinalState_ErrorHandling tests the fixed error handling
// in the retryUntilAAPJobReachesAnyFinalState function. This validates that:
// 1. Diagnostics errors from client.Get() are properly handled (not standard Go errors)
// 2. The function returns retryable errors for transient failures
// 3. Model state is updated correctly as jobs transition from non-final to final states
func TestRetryUntilAAPJobReachesAnyFinalState_ErrorHandling(t *testing.T) {
	t.Parallel()

	// Test diagnostics error handling (500 server error)
	t.Run("handles diagnostics errors", func(t *testing.T) {
		mockClient := NewMockHTTPClient([]string{"GET"}, 500) // Server error
		var status string = statusPendingConst
		retryFunc := retryUntilAAPJobReachesAnyFinalState(t.Context(), mockClient, "/api/v2/jobs/999/", &status)
		err := retryFunc()

		// Should return a retryable error due to 500 status
		if err == nil {
			t.Errorf("expected error but got none")
		}
		// The retry function should return a retry.RetryError
		errStr := fmt.Sprintf("%v", err)
		if !strings.Contains(errStr, "error fetching job status") {
			t.Errorf("expected error to contain 'error fetching job status', got: %v", errStr)
		}

		// Model state should remain unchanged since parsing never succeeded
		if status != statusPendingConst {
			t.Errorf("expected status to remain 'pending' after error, got '%s'", status)
		}
	})

	// Test that non-final state returns retryable error
	t.Run("returns retryable error for non-final state", func(t *testing.T) {
		mockClient := NewMockHTTPClient([]string{"GET"}, 200)
		var status string
		retryFunc := retryUntilAAPJobReachesAnyFinalState(t.Context(), mockClient, "/api/v2/jobs/1/", &status)
		err := retryFunc()

		// Should return retryable error since "running" is not a final state
		if err == nil {
			t.Errorf("expected error but got none")
		}
		// Status should be updated with "running" status from mock response
		if status != "running" {
			t.Errorf("expected status 'running', got '%s'", status)
		}
		// Error should indicate non-final state
		errStr := fmt.Sprintf("%v", err)
		if !strings.Contains(errStr, "hasn't yet reached a final state") {
			t.Errorf("expected error to contain 'hasn't yet reached a final state', got: %v", errStr)
		}
	})

	// Test job state transition from running to successful
	t.Run("handles job state transition from running to successful", func(t *testing.T) {
		// Configure mock responses: first call returns "running", subsequent calls return "successful"
		responses := []MockResponse{
			{
				Data:        []byte(`{"status": "running", "url": "/api/v2/jobs/123/", "type": "run"}`),
				Diagnostics: diag.Diagnostics{},
			},
			{
				Data:        []byte(`{"status": "successful", "url": "/api/v2/jobs/123/", "type": "run"}`),
				Diagnostics: diag.Diagnostics{},
			},
		}
		mockClient := NewConfigurableSequenceMockClient(responses)
		var status string
		retryFunc := retryUntilAAPJobReachesAnyFinalState(t.Context(), mockClient, "/api/v2/jobs/123/", &status)

		// First call - job should be running (returns retryable error)
		err1 := retryFunc()
		if err1 == nil {
			t.Errorf("expected retryable error for running job but got none")
		}
		if status != statusRunningConst {
			t.Errorf("expected status 'running' after first call, got '%s'", status)
		}

		// Second call - job should be successful (returns no error)
		err2 := retryFunc()
		if err2 != nil {
			t.Errorf("expected no error for successful job but got: %v", err2)
		}
		if status != "successful" {
			t.Errorf("expected status 'successful' after second call, got '%s'", status)
		}
	})
}

// MockResponse represents a single response in a sequence
type MockResponse struct {
	Data        []byte
	Diagnostics diag.Diagnostics
}

// ConfigurableSequenceMockClient allows configuring a sequence of responses for multi-call scenarios
// This is useful for testing retry logic, state transitions, and other multi-step operations
type ConfigurableSequenceMockClient struct {
	callCount *int
	responses []MockResponse
}

// NewConfigurableSequenceMockClient creates a new mock client with a predefined sequence of responses
func NewConfigurableSequenceMockClient(responses []MockResponse) *ConfigurableSequenceMockClient {
	callCount := 0
	return &ConfigurableSequenceMockClient{
		callCount: &callCount,
		responses: responses,
	}
}

func (m *ConfigurableSequenceMockClient) Get(_ string) ([]byte, diag.Diagnostics) {
	*m.callCount++

	// Return the response for this call number (1-indexed)
	if *m.callCount <= len(m.responses) {
		response := m.responses[*m.callCount-1]
		return response.Data, response.Diagnostics
	}

	// If we've run out of configured responses, return the last one
	// This handles cases where retry logic might make more calls than expected
	if len(m.responses) > 0 {
		lastResponse := m.responses[len(m.responses)-1]
		return lastResponse.Data, lastResponse.Diagnostics
	}

	// Fallback: return empty response
	return []byte(`{}`), diag.Diagnostics{}
}

func (m *ConfigurableSequenceMockClient) GetWithParams(path string, _ map[string]string) ([]byte, diag.Diagnostics) {
	return m.Get(path)
}

// Stub implementations for the remaining interface methods
func (m *ConfigurableSequenceMockClient) Create(_ string, _ io.Reader) ([]byte, diag.Diagnostics) {
	return nil, diag.Diagnostics{}
}

func (m *ConfigurableSequenceMockClient) Update(_ string, _ io.Reader) ([]byte, diag.Diagnostics) {
	return nil, diag.Diagnostics{}
}

func (m *ConfigurableSequenceMockClient) Delete(_ string) ([]byte, diag.Diagnostics) {
	return nil, diag.Diagnostics{}
}

func (m *ConfigurableSequenceMockClient) GetWithStatus(path string, _ map[string]string) ([]byte, diag.Diagnostics, int) {
	body, diags := m.Get(path)
	return body, diags, 200
}

func (m *ConfigurableSequenceMockClient) UpdateWithStatus(_ string, _ io.Reader) ([]byte, diag.Diagnostics, int) {
	return nil, diag.Diagnostics{}, 200
}

func (m *ConfigurableSequenceMockClient) DeleteWithStatus(_ string) ([]byte, diag.Diagnostics, int) {
	return nil, diag.Diagnostics{}, 204
}

func (m *ConfigurableSequenceMockClient) doRequest(_ string, _ string, _ map[string]string, _ io.Reader) (*http.Response, []byte, error) {
	return nil, nil, nil
}

func (m *ConfigurableSequenceMockClient) setApiEndpoint() diag.Diagnostics {
	return diag.Diagnostics{}
}

func (m *ConfigurableSequenceMockClient) getApiEndpoint() string {
	return "/api/v2"
}

func (m *ConfigurableSequenceMockClient) getEdaApiEndpoint() string {
	return "/api/eda/v1"
}

// assertLogFieldEquals validates a specific field in the parsed log entry
func assertLogFieldEquals(t *testing.T, logEntry map[string]interface{}, fieldName string, expectedValue interface{}) {
	t.Helper()

	// Check if field exists
	actualValue, exists := logEntry[fieldName]
	if !exists {
		t.Errorf("Expected field '%s' not found in structured log", fieldName)
		return
	}

	// Handle JSON number conversion (JSON numbers become float64)
	if expectedInt, ok := expectedValue.(int); ok {
		expectedValue = float64(expectedInt)
	}

	// Compare values
	if actualValue != expectedValue {
		t.Errorf("Field '%s': expected '%v', got '%v'", fieldName, expectedValue, actualValue)
	}
}

// TestRetryUntilAAPJobReachesAnyFinalState_LoggingBehavior ensures tflog.Debug is used
// with expected structured fields instead of fmt.Printf
func TestRetryUntilAAPJobReachesAnyFinalState_LoggingBehavior(t *testing.T) {
	// Create a buffer to capture tflog output
	var logBuffer strings.Builder

	// Create a context with tflog writing to our buffer
	ctx := tflogtest.RootLogger(t.Context(), &logBuffer)

	// Create test model with known values for verification
	model := &JobResourceModel{JobModel: JobModel{
		TemplateID: types.Int64Value(0), // Mock doesn't include job_template here
	},
		URL:    types.StringValue("/api/v2/jobs/1/"), // This path exists in MockConfig
		Status: types.StringValue("pending"),         // Will be updated by ParseHttpResponse
	}

	// Create a custom mock response for this test (avoid modifying shared fixtures)
	testJobResponse := map[string]string{
		"status": "running",
		"type":   "check",
		"url":    "/api/v2/jobs/1/",
	}

	// Create a mock client with custom config for this test
	mockClient := NewMockHTTPClient([]string{"GET"}, 200)

	// Add our test response to the mock config temporarily
	originalResponse := MockConfig["/api/v2/jobs/1/"]
	MockConfig["/api/v2/jobs/1/"] = testJobResponse
	defer func() {
		MockConfig["/api/v2/jobs/1/"] = originalResponse
	}()

	// Execute the retry function once (should return retryable error since "running" is not final)
	var status string
	retryFunc := retryUntilAAPJobReachesAnyFinalState(ctx, mockClient, model.URL.ValueString(), &status)
	err := retryFunc()

	// Verify we get a retryable error since "running" is not a final state
	if err == nil {
		t.Error("Expected retryable error for non-final state, got nil")
	}

	// Verify the model was updated to "running" status from mock response
	if status != statusRunningConst {
		t.Errorf("Expected model status to be updated to 'running', got '%s'", status)
	}

	// Check the captured tflog output
	logOutput := logBuffer.String()
	if len(logOutput) == 0 {
		t.Fatal("No tflog output captured - tflog.Debug may not have been called")
	}

	// Parse the structured JSON log output once
	var logEntry map[string]interface{}
	parseErr := json.Unmarshal([]byte(strings.TrimSpace(logOutput)), &logEntry)
	if parseErr != nil {
		t.Fatalf("Failed to parse tflog output as JSON: %v\nOutput: %s", parseErr, logOutput)
	}

	// Validate structured log fields using helper function
	assertLogFieldEquals(t, logEntry, "@level", "debug")
	assertLogFieldEquals(t, logEntry, "@message", "Job status update")
	assertLogFieldEquals(t, logEntry, "@module", "provider")
	assertLogFieldEquals(t, logEntry, "status", statusRunningConst)
}
