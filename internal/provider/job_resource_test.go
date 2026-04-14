package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
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
	"github.com/hashicorp/terraform-plugin-log/tflogtest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"go.uber.org/mock/gomock"
)

const (
	statusRunningConst = "running"
	statusPendingConst = "pending"
)

// createMockResponse creates an http.Response with the required Request field for ValidateResponse
func createMockResponse(statusCode int, method, urlPath string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Request: &http.Request{
			Method: method,
			URL:    &url.URL{Path: urlPath},
		},
	}
}

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

func TestJobResourceParseHTTPResponse(t *testing.T) {
	templateID := basetypes.NewInt64Value(1)
	inventoryID := basetypes.NewInt64Value(2)
	extraVars := customtypes.NewAAPCustomStringNull()
	// Optional+Computed fields are now set from API response values.
	// UseStateForUnknown() plan modifiers handle drift prevention at plan time.
	limit := customtypes.NewAAPCustomStringValue("")
	jobTags := customtypes.NewAAPCustomStringValue("")
	skipTags := customtypes.NewAAPCustomStringValue("")
	diffMode := types.BoolValue(false)
	verbosity := types.Int64Value(0)
	executionEnvironmentID := types.Int64Value(0)
	forks := types.Int64Value(0)
	jobSliceCount := types.Int64Value(0)
	timeout := types.Int64Value(0)
	instanceGroups := types.ListNull(types.Int64Type)
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
				TemplateID:             templateID,
				InventoryID:            inventoryID,
				ExtraVars:              extraVars,
				Limit:                  limit,
				JobTags:                jobTags,
				SkipTags:               skipTags,
				DiffMode:               diffMode,
				Verbosity:              verbosity,
				ExecutionEnvironmentID: executionEnvironmentID,
				Forks:                  forks,
				JobSliceCount:          jobSliceCount,
				Timeout:                timeout,
				InstanceGroups:         instanceGroups,
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
				TemplateID:             templateID,
				InventoryID:            inventoryID,
				ExtraVars:              extraVars,
				Limit:                  limit,
				JobTags:                jobTags,
				SkipTags:               skipTags,
				DiffMode:               diffMode,
				Verbosity:              verbosity,
				ExecutionEnvironmentID: executionEnvironmentID,
				Forks:                  forks,
				JobSliceCount:          jobSliceCount,
				Timeout:                timeout,
				InstanceGroups:         instanceGroups,
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
			diags := resource.ParseHTTPResponse(test.input)
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
		"AAP_TEST_JOB_TEMPLATE_FAIL_ID",
		"AAP_TEST_WORKFLOW_JOB_TEMPLATE_ID",
		"AAP_TEST_WORKFLOW_JOB_TEMPLATE_FAIL_ID",
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
	jobTemplateID := os.Getenv("AAP_TEST_JOB_TEMPLATE_INVENTORY_PROMPT_ID")
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
		var jobAPIModel JobAPIModel
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
			err = json.Unmarshal(body, &jobAPIModel)
			if err != nil {
				return retry.NonRetryableError(err)
			}
			if IsFinalStateAAPJob(jobAPIModel.Status) {
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
	var jobURL string

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
					testAccCheckJobUpdate(&jobURL, false),
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
					testAccDeleteJob(&jobURL),
				),
				ExpectNonEmptyPlan: true,
			},
			// Apply the plan again and confirm the job is re-created with a different URL.
			{
				Config: testAccBasicJob(jobTemplateID),
				Check: resource.ComposeAggregateTestCheckFunc(
					checkBasicJobAttributes(t, resourceNameJob, reJobStatus),
					testAccCheckJobUpdate(&jobURL, true),
				),
			},
		},
	})
}

func testAccDeleteJob(jobURL *string) func(s *terraform.State) error {
	return func(_ *terraform.State) error {
		_, err := testDeleteResource(*jobURL)
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
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockClient := NewMockProviderHTTPClient(ctrl)
		errorDiags := diag.Diagnostics{}
		errorDiags.AddError("Server Error", "Internal server error")
		mockClient.EXPECT().Get("/api/v2/jobs/999/").Return(nil, errorDiags)

		var status = statusPendingConst
		retryProgressFunc := func(status string) {
			t.Logf("Job status: %s", status)
		}
		retryFunc := retryUntilAAPJobReachesAnyFinalState(t.Context(), mockClient, retryProgressFunc, "/api/v2/jobs/999/", &status)
		err := retryFunc()

		// Should return a retryable error due to 500 status
		if err == nil {
			t.Errorf("expected error but got none")
		}
		// The retry function should return a retry.retry.RetryError
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
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockClient := NewMockProviderHTTPClient(ctrl)
		mockResponse := []byte(`{"status": "running", "url": "/api/v2/jobs/1/", "type": "run"}`)
		mockClient.EXPECT().Get("/api/v2/jobs/1/").Return(mockResponse, diag.Diagnostics{})

		var status string
		retryProgressFunc := func(status string) {
			t.Logf("Job status: %s", status)
		}
		retryFunc := retryUntilAAPJobReachesAnyFinalState(t.Context(), mockClient, retryProgressFunc, "/api/v2/jobs/1/", &status)
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
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockClient := NewMockProviderHTTPClient(ctrl)
		// Configure mock responses: first call returns "running", second call returns "successful"
		runningResponse := []byte(`{"status": "running", "url": "/api/v2/jobs/123/", "type": "run"}`)
		successfulResponse := []byte(`{"status": "successful", "url": "/api/v2/jobs/123/", "type": "run"}`)
		mockClient.EXPECT().Get("/api/v2/jobs/123/").Return(runningResponse, diag.Diagnostics{}).Times(1)
		mockClient.EXPECT().Get("/api/v2/jobs/123/").Return(successfulResponse, diag.Diagnostics{}).Times(1)

		var status string
		retryProgressFunc := func(status string) {
			t.Logf("Job status: %s", status)
		}
		retryFunc := retryUntilAAPJobReachesAnyFinalState(t.Context(), mockClient, retryProgressFunc, "/api/v2/jobs/123/", &status)

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
		if status != statusSuccessfulConst {
			t.Errorf("expected status 'successful' after second call, got '%s'", status)
		}
	})
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
		URL:    types.StringValue("/api/v2/jobs/1/"),
		Status: types.StringValue("pending"), // Will be updated by ParseHttpResponse
	}

	// Create a mock client with gomock
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := NewMockProviderHTTPClient(ctrl)
	mockResponse := []byte(`{"status": "running", "type": "check", "url": "/api/v2/jobs/1/"}`)
	mockClient.EXPECT().Get("/api/v2/jobs/1/").Return(mockResponse, diag.Diagnostics{})

	// Execute the retry function once (should return retryable error since "running" is not final)
	var status string
	retryProgressFunc := func(status string) {
		t.Logf("Job status: %s", status)
	}
	retryFunc := retryUntilAAPJobReachesAnyFinalState(ctx, mockClient, retryProgressFunc, model.URL.ValueString(), &status)
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

// TestAccAAPJob_AllFieldsOnPrompt tests that a job resource with all fields on prompt
// can be launched successfully when all required fields are provided.
func TestAccAAPJob_AllFieldsOnPrompt(t *testing.T) {
	jobTemplateID := os.Getenv("AAP_TEST_JOB_TEMPLATE_ALL_FIELDS_PROMPT_ID")
	if jobTemplateID == "" {
		t.Skip("AAP_TEST_JOB_TEMPLATE_ALL_FIELDS_PROMPT_ID environment variable not set")
	}
	credentialID := os.Getenv("AAP_TEST_DEMO_CREDENTIAL_ID")
	if credentialID == "" {
		t.Skip("AAP_TEST_DEMO_CREDENTIAL_ID environment variable not set")
	}
	labelID := os.Getenv("AAP_TEST_LABEL_ID")
	if labelID == "" {
		t.Skip("AAP_TEST_LABEL_ID environment variable not set")
	}
	inventoryName := acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)
	ctx := t.Context()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccJobResourcePreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccJobAllFieldsOnPrompt(inventoryName, jobTemplateID, credentialID, labelID),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckJobExists,
					testAccCheckJobPause(ctx, resourceNameJob),
				),
			},
		},
	})
}

// TestAccAAPJob_AllFieldsOnPrompt_MissingRequired tests that a job resource with all
// fields on prompt fails when required fields are not provided.
func TestAccAAPJob_AllFieldsOnPrompt_MissingRequired(t *testing.T) {
	jobTemplateID := os.Getenv("AAP_TEST_JOB_TEMPLATE_ALL_FIELDS_PROMPT_ID")
	if jobTemplateID == "" {
		t.Skip("AAP_TEST_JOB_TEMPLATE_ALL_FIELDS_PROMPT_ID environment variable not set")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccJobResourcePreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccJobAllFieldsOnPromptMissingRequired(jobTemplateID),
				ExpectError: regexp.MustCompile(".*Missing required field.*"),
			},
		},
	})
}

func testAccJobAllFieldsOnPrompt(inventoryName, jobTemplateID, credentialID, labelID string) string {
	return fmt.Sprintf(`
resource "aap_inventory" "test" {
	name = "%s"
}

resource "aap_job" "test" {
	job_template_id       = %s
	inventory_id          = aap_inventory.test.id
	credentials           = [%s]
	labels                = [%s]
	extra_vars            = "{\"test_var\": \"test_value\"}"
	limit                 = "localhost"
	job_tags              = "test"
	skip_tags             = "skip"
	diff_mode             = true
	verbosity             = 1
	forks                 = 5
	job_slice_count       = 1
	timeout               = 300
	wait_for_completion   = true
}
`, inventoryName, jobTemplateID, credentialID, labelID)
}

func testAccJobAllFieldsOnPromptMissingRequired(jobTemplateID string) string {
	return fmt.Sprintf(`
resource "aap_job" "test" {
	job_template_id = %s
}
`, jobTemplateID)
}

func TestJobModelCreateRequestBody(t *testing.T) {
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
		{
			name: "credentials serialization",
			input: JobResourceModel{JobModel: JobModel{
				TemplateID:  types.Int64Value(1),
				Credentials: basetypes.NewListValueMust(types.Int64Type, []attr.Value{types.Int64Value(1), types.Int64Value(2)}),
			}},
			expected: []byte(`{"credentials":[1,2]}`),
		},
		{
			name: "labels serialization",
			input: JobResourceModel{JobModel: JobModel{
				TemplateID: types.Int64Value(1),
				Labels:     basetypes.NewListValueMust(types.Int64Type, []attr.Value{types.Int64Value(10), types.Int64Value(20)}),
			}},
			expected: []byte(`{"labels":[10,20]}`),
		},
		{
			name: "instance groups serialization",
			input: JobResourceModel{JobModel: JobModel{
				TemplateID:     types.Int64Value(1),
				InstanceGroups: basetypes.NewListValueMust(types.Int64Type, []attr.Value{types.Int64Value(2), types.Int64Value(3), types.Int64Value(4)}),
			}},
			expected: []byte(`{"instance_groups":[2,3,4]}`),
		},
		{
			name: "all prompt-on-launch fields",
			input: JobResourceModel{JobModel: JobModel{
				TemplateID:     types.Int64Value(1),
				InventoryID:    basetypes.NewInt64Value(100),
				Credentials:    basetypes.NewListValueMust(types.Int64Type, []attr.Value{types.Int64Value(1)}),
				Labels:         basetypes.NewListValueMust(types.Int64Type, []attr.Value{types.Int64Value(5)}),
				InstanceGroups: basetypes.NewListValueMust(types.Int64Type, []attr.Value{types.Int64Value(2)}),
				ExtraVars:      customtypes.NewAAPCustomStringValue(`{"key":"value"}`),
				Limit:          customtypes.NewAAPCustomStringValue("webservers"),
				Verbosity:      basetypes.NewInt64Value(2),
				DiffMode:       basetypes.NewBoolValue(true),
			}},
			expected: []byte(`{"inventory":100,"extra_vars":"{\"key\":\"value\"}","limit":"webservers",` +
				`"diff_mode":true,"verbosity":2,"instance_groups":[2],"credentials":[1],"labels":[5]}`),
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

func TestJobModelGetLaunchJob(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		templateID     int64
		mockResponse   []byte
		mockStatusCode int
		expectError    bool
		validateResult func(t *testing.T, config JobLaunchAPIModel)
	}{
		{
			name:           "successful retrieval of launch config",
			templateID:     123,
			mockStatusCode: http.StatusOK,
			mockResponse: []byte(`{
				"ask_variables_on_launch": true,
				"ask_tags_on_launch": false,
				"ask_skip_tags_on_launch": false,
				"ask_limit_on_launch": true,
				"ask_inventory_on_launch": true,
				"ask_verbosity_on_launch": true
			}`),
			expectError: false,
			validateResult: func(t *testing.T, config JobLaunchAPIModel) {
				if !config.AskVariablesOnLaunch {
					t.Error("expected AskVariablesOnLaunch to be true")
				}
				if !config.AskLimitOnLaunch {
					t.Error("expected AskLimitOnLaunch to be true")
				}
				if config.AskTagsOnLaunch {
					t.Error("expected AskTagsOnLaunch to be false")
				}
			},
		},
		{
			name:           "handles API 404 error",
			templateID:     999,
			mockStatusCode: http.StatusNotFound,
			mockResponse:   []byte(`{"detail": "Not found."}`),
			expectError:    true,
		},
		{
			name:           "handles invalid JSON response",
			templateID:     123,
			mockStatusCode: http.StatusOK,
			mockResponse:   []byte(`not valid json`),
			expectError:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := NewMockProviderHTTPClient(ctrl)
			mockClient.EXPECT().getAPIEndpoint().Return("/api/v2")

			var resp *http.Response
			if tc.mockStatusCode != http.StatusOK {
				resp = createMockResponse(tc.mockStatusCode, http.MethodGet, "/api/v2/job_templates/"+types.Int64Value(tc.templateID).String()+"/launch")
			} else {
				resp = &http.Response{StatusCode: tc.mockStatusCode}
			}

			mockClient.EXPECT().
				doRequest(http.MethodGet, gomock.Any(), nil, nil).
				Return(resp, tc.mockResponse, nil)

			model := &JobModel{TemplateID: types.Int64Value(tc.templateID)}
			config, diags := model.GetLaunchJob(mockClient)

			if tc.expectError && !diags.HasError() {
				t.Error("expected error but got none")
			}
			if !tc.expectError && diags.HasError() {
				t.Errorf("unexpected error: %v", diags.Errors())
			}
			if tc.validateResult != nil && !diags.HasError() {
				tc.validateResult(t, config)
			}
		})
	}
}

func TestJobModelCanJobBeLaunched(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		launchConfig   JobLaunchAPIModel
		model          JobModel
		expectError    bool
		expectWarnings bool
	}{
		// Base case
		{
			name:         "all fields optional - no errors",
			launchConfig: JobLaunchAPIModel{},
			model:        JobModel{TemplateID: types.Int64Value(1)},
			expectError:  false,
		},
		// extra_vars
		{
			name:         "extra_vars required but not provided",
			launchConfig: JobLaunchAPIModel{AskVariablesOnLaunch: true},
			model:        JobModel{TemplateID: types.Int64Value(1), ExtraVars: customtypes.NewAAPCustomStringNull()},
			expectError:  true,
		},
		{
			name:           "extra_vars provided but not expected - warning",
			launchConfig:   JobLaunchAPIModel{AskVariablesOnLaunch: false},
			model:          JobModel{TemplateID: types.Int64Value(1), ExtraVars: customtypes.NewAAPCustomStringValue(`{"key": "value"}`)},
			expectWarnings: true,
		},
		// inventory_id
		{
			name:         "inventory_id required but not provided",
			launchConfig: JobLaunchAPIModel{AskInventoryOnLaunch: true},
			model:        JobModel{TemplateID: types.Int64Value(1), InventoryID: types.Int64Null()},
			expectError:  true,
		},
		{
			name:           "inventory_id provided but not expected - warning",
			launchConfig:   JobLaunchAPIModel{AskInventoryOnLaunch: false},
			model:          JobModel{TemplateID: types.Int64Value(1), InventoryID: types.Int64Value(10)},
			expectWarnings: true,
		},
		// limit
		{
			name:         "limit required but not provided",
			launchConfig: JobLaunchAPIModel{AskLimitOnLaunch: true},
			model:        JobModel{TemplateID: types.Int64Value(1), Limit: customtypes.NewAAPCustomStringNull()},
			expectError:  true,
		},
		{
			name:           "limit provided but not expected - warning",
			launchConfig:   JobLaunchAPIModel{AskLimitOnLaunch: false},
			model:          JobModel{TemplateID: types.Int64Value(1), Limit: customtypes.NewAAPCustomStringValue("all")},
			expectWarnings: true,
		},
		// job_tags
		{
			name:         "job_tags required but not provided",
			launchConfig: JobLaunchAPIModel{AskTagsOnLaunch: true},
			model:        JobModel{TemplateID: types.Int64Value(1), JobTags: customtypes.NewAAPCustomStringNull()},
			expectError:  true,
		},
		{
			name:           "job_tags provided but not expected - warning",
			launchConfig:   JobLaunchAPIModel{AskTagsOnLaunch: false},
			model:          JobModel{TemplateID: types.Int64Value(1), JobTags: customtypes.NewAAPCustomStringValue("deploy")},
			expectWarnings: true,
		},
		// skip_tags
		{
			name:         "skip_tags required but not provided",
			launchConfig: JobLaunchAPIModel{AskSkipTagsOnLaunch: true},
			model:        JobModel{TemplateID: types.Int64Value(1), SkipTags: customtypes.NewAAPCustomStringNull()},
			expectError:  true,
		},
		{
			name:           "skip_tags provided but not expected - warning",
			launchConfig:   JobLaunchAPIModel{AskSkipTagsOnLaunch: false},
			model:          JobModel{TemplateID: types.Int64Value(1), SkipTags: customtypes.NewAAPCustomStringValue("debug")},
			expectWarnings: true,
		},
		// diff_mode
		{
			name:         "diff_mode required but not provided",
			launchConfig: JobLaunchAPIModel{AskDiffModeOnLaunch: true},
			model:        JobModel{TemplateID: types.Int64Value(1), DiffMode: types.BoolNull()},
			expectError:  true,
		},
		{
			name:           "diff_mode provided but not expected - warning",
			launchConfig:   JobLaunchAPIModel{AskDiffModeOnLaunch: false},
			model:          JobModel{TemplateID: types.Int64Value(1), DiffMode: types.BoolValue(true)},
			expectWarnings: true,
		},
		// verbosity
		{
			name:         "verbosity required but not provided",
			launchConfig: JobLaunchAPIModel{AskVerbosityOnLaunch: true},
			model:        JobModel{TemplateID: types.Int64Value(1), Verbosity: types.Int64Null()},
			expectError:  true,
		},
		{
			name:           "verbosity provided but not expected - warning",
			launchConfig:   JobLaunchAPIModel{AskVerbosityOnLaunch: false},
			model:          JobModel{TemplateID: types.Int64Value(1), Verbosity: types.Int64Value(3)},
			expectWarnings: true,
		},
		// forks
		{
			name:         "forks required but not provided",
			launchConfig: JobLaunchAPIModel{AskForksOnLaunch: true},
			model:        JobModel{TemplateID: types.Int64Value(1), Forks: types.Int64Null()},
			expectError:  true,
		},
		{
			name:           "forks provided but not expected - warning",
			launchConfig:   JobLaunchAPIModel{AskForksOnLaunch: false},
			model:          JobModel{TemplateID: types.Int64Value(1), Forks: types.Int64Value(10)},
			expectWarnings: true,
		},
		// timeout
		{
			name:         "timeout required but not provided",
			launchConfig: JobLaunchAPIModel{AskTimeoutOnLaunch: true},
			model:        JobModel{TemplateID: types.Int64Value(1), Timeout: types.Int64Null()},
			expectError:  true,
		},
		{
			name:           "timeout provided but not expected - warning",
			launchConfig:   JobLaunchAPIModel{AskTimeoutOnLaunch: false},
			model:          JobModel{TemplateID: types.Int64Value(1), Timeout: types.Int64Value(3600)},
			expectWarnings: true,
		},
		// job_slice_count
		{
			name:         "job_slice_count required but not provided",
			launchConfig: JobLaunchAPIModel{AskJobSliceCountOnLaunch: true},
			model:        JobModel{TemplateID: types.Int64Value(1), JobSliceCount: types.Int64Null()},
			expectError:  true,
		},
		{
			name:           "job_slice_count provided but not expected - warning",
			launchConfig:   JobLaunchAPIModel{AskJobSliceCountOnLaunch: false},
			model:          JobModel{TemplateID: types.Int64Value(1), JobSliceCount: types.Int64Value(4)},
			expectWarnings: true,
		},
		// execution_environment
		{
			name:         "execution_environment required but not provided",
			launchConfig: JobLaunchAPIModel{AskExecutionEnvironmentOnLaunch: true},
			model:        JobModel{TemplateID: types.Int64Value(1), ExecutionEnvironmentID: types.Int64Null()},
			expectError:  true,
		},
		{
			name:           "execution_environment provided but not expected - warning",
			launchConfig:   JobLaunchAPIModel{AskExecutionEnvironmentOnLaunch: false},
			model:          JobModel{TemplateID: types.Int64Value(1), ExecutionEnvironmentID: types.Int64Value(5)},
			expectWarnings: true,
		},
		// instance_groups
		{
			name:         "instance_groups required but not provided",
			launchConfig: JobLaunchAPIModel{AskInstanceGroupsOnLaunch: true},
			model:        JobModel{TemplateID: types.Int64Value(1), InstanceGroups: types.ListNull(types.Int64Type)},
			expectError:  true,
		},
		{
			name:           "instance_groups provided but not expected - warning",
			launchConfig:   JobLaunchAPIModel{AskInstanceGroupsOnLaunch: false},
			model:          JobModel{TemplateID: types.Int64Value(1), InstanceGroups: basetypes.NewListValueMust(types.Int64Type, []attr.Value{types.Int64Value(5)})},
			expectWarnings: true,
		},
		// credentials
		{
			name:         "credentials required but not provided",
			launchConfig: JobLaunchAPIModel{AskCredentialOnLaunch: true},
			model:        JobModel{TemplateID: types.Int64Value(1), Credentials: types.ListNull(types.Int64Type)},
			expectError:  true,
		},
		{
			name:           "credentials provided but not expected - warning",
			launchConfig:   JobLaunchAPIModel{AskCredentialOnLaunch: false},
			model:          JobModel{TemplateID: types.Int64Value(1), Credentials: basetypes.NewListValueMust(types.Int64Type, []attr.Value{types.Int64Value(1)})},
			expectWarnings: true,
		},
		// labels
		{
			name:         "labels required but not provided",
			launchConfig: JobLaunchAPIModel{AskLabelsOnLaunch: true},
			model:        JobModel{TemplateID: types.Int64Value(1), Labels: types.ListNull(types.Int64Type)},
			expectError:  true,
		},
		{
			name:           "labels provided but not expected - warning",
			launchConfig:   JobLaunchAPIModel{AskLabelsOnLaunch: false},
			model:          JobModel{TemplateID: types.Int64Value(1), Labels: basetypes.NewListValueMust(types.Int64Type, []attr.Value{types.Int64Value(1)})},
			expectWarnings: true,
		},
		// Combined success case
		{
			name: "all required fields provided - no errors",
			launchConfig: JobLaunchAPIModel{
				AskVariablesOnLaunch:      true,
				AskLimitOnLaunch:          true,
				AskInventoryOnLaunch:      true,
				AskVerbosityOnLaunch:      true,
				AskInstanceGroupsOnLaunch: true,
			},
			model: JobModel{
				TemplateID:     types.Int64Value(1),
				ExtraVars:      customtypes.NewAAPCustomStringValue(`{"key": "value"}`),
				Limit:          customtypes.NewAAPCustomStringValue("all"),
				InventoryID:    types.Int64Value(10),
				Verbosity:      types.Int64Value(2),
				InstanceGroups: basetypes.NewListValueMust(types.Int64Type, []attr.Value{types.Int64Value(5)}),
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := NewMockProviderHTTPClient(ctrl)
			mockClient.EXPECT().getAPIEndpoint().Return("/api/v2")

			configJSON, _ := json.Marshal(tc.launchConfig)
			mockClient.EXPECT().
				doRequest(http.MethodGet, gomock.Any(), nil, nil).
				Return(&http.Response{StatusCode: http.StatusOK}, configJSON, nil)

			diags := tc.model.CanJobBeLaunched(mockClient)

			if tc.expectError && !diags.HasError() {
				t.Error("expected error but got none")
			}
			if !tc.expectError && diags.HasError() {
				t.Errorf("unexpected error: %v", diags.Errors())
			}
			if tc.expectWarnings && len(diags.Warnings()) == 0 {
				t.Error("expected warnings but got none")
			}
		})
	}
}

func TestJobModelLaunchJob(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		model          JobModel
		launchConfig   JobLaunchAPIModel
		postStatusCode int
		postResponse   []byte
		expectError    bool
		skipPostMock   bool // If CanJobBeLaunched fails, POST is not called
	}{
		{
			name:         "successful job launch",
			model:        JobModel{TemplateID: types.Int64Value(123)},
			launchConfig: JobLaunchAPIModel{
				// All fields optional
			},
			postStatusCode: http.StatusCreated,
			postResponse: []byte(`{
				"id": 456,
				"url": "/api/v2/jobs/456/",
				"status": "pending"
			}`),
			expectError: false,
		},
		{
			name: "launch fails when CanJobBeLaunched fails",
			model: JobModel{
				TemplateID: types.Int64Value(123),
				ExtraVars:  customtypes.NewAAPCustomStringNull(),
			},
			launchConfig: JobLaunchAPIModel{
				AskVariablesOnLaunch: true, // extra_vars required but not provided
			},
			expectError:  true,
			skipPostMock: true,
		},
		{
			name:           "launch fails when POST fails",
			model:          JobModel{TemplateID: types.Int64Value(123)},
			launchConfig:   JobLaunchAPIModel{},
			postStatusCode: http.StatusInternalServerError,
			postResponse:   []byte(`{"error": "server error"}`),
			expectError:    true,
		},
		{
			name: "launch with all parameters",
			model: JobModel{
				TemplateID:  types.Int64Value(123),
				InventoryID: types.Int64Value(10),
				ExtraVars:   customtypes.NewAAPCustomStringValue(`{"env": "prod"}`),
				Limit:       customtypes.NewAAPCustomStringValue("webservers"),
				Verbosity:   types.Int64Value(3),
			},
			launchConfig: JobLaunchAPIModel{
				AskVariablesOnLaunch: true,
				AskLimitOnLaunch:     true,
				AskInventoryOnLaunch: true,
				AskVerbosityOnLaunch: true,
			},
			postStatusCode: http.StatusCreated,
			postResponse: []byte(`{
				"id": 789,
				"url": "/api/v2/jobs/789/",
				"status": "pending"
			}`),
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := NewMockProviderHTTPClient(ctrl)

			// GetLaunchJob mock
			expectedAPICalls := 1
			if !tc.skipPostMock {
				expectedAPICalls = 2
			}
			mockClient.EXPECT().getAPIEndpoint().Return("/api/v2").Times(expectedAPICalls)

			configJSON, _ := json.Marshal(tc.launchConfig)
			mockClient.EXPECT().
				doRequest(http.MethodGet, gomock.Any(), nil, nil).
				Return(&http.Response{StatusCode: http.StatusOK}, configJSON, nil)

			// POST mock (only if CanJobBeLaunched passes)
			if !tc.skipPostMock {
				var postResp *http.Response
				if tc.postStatusCode != http.StatusCreated {
					postResp = createMockResponse(tc.postStatusCode, http.MethodPost, "/api/v2/job_templates/123/launch")
				} else {
					postResp = &http.Response{StatusCode: tc.postStatusCode}
				}
				mockClient.EXPECT().
					doRequest(http.MethodPost, gomock.Any(), nil, gomock.Any()).
					Return(postResp, tc.postResponse, nil)
			}

			body, diags := tc.model.LaunchJob(mockClient)

			if tc.expectError && !diags.HasError() {
				t.Error("expected error but got none")
			}
			if !tc.expectError && diags.HasError() {
				t.Errorf("unexpected error: %v", diags.Errors())
			}
			if tc.expectError && body != nil {
				t.Error("expected nil body on error")
			}
			if !tc.expectError && body == nil {
				t.Error("expected response body, got nil")
			}
		})
	}
}
