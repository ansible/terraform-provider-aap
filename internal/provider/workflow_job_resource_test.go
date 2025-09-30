package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"testing"
	"time"

	"github.com/ansible/terraform-provider-aap/internal/provider/customtypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestWorkflowJobResourceSchema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schemaRequest := fwresource.SchemaRequest{}
	schemaResponse := &fwresource.SchemaResponse{}

	// Instantiate the WorkflowJobResource and call its Schema method
	NewWorkflowJobResource().Schema(ctx, schemaRequest, schemaResponse)

	if schemaResponse.Diagnostics.HasError() {
		t.Fatalf("Schema method diagnostics: %+v", schemaResponse.Diagnostics)
	}

	// Validate the schema
	diagnostics := schemaResponse.Schema.ValidateImplementation(ctx)

	if diagnostics.HasError() {
		t.Fatalf("Schema validation diagnostics: %+v", diagnostics)
	}
}

func TestWorkflowJobResourceCreateRequestBody(t *testing.T) {
	testTable := []struct {
		name     string
		input    WorkflowJobResourceModel
		expected []byte
	}{
		{
			name: "unknown values",
			input: WorkflowJobResourceModel{
				ExtraVars:   customtypes.NewAAPCustomStringUnknown(),
				InventoryID: basetypes.NewInt64Unknown(),
				TemplateID:  types.Int64Value(1),
			},
			expected: []byte(`{}`),
		},
		{
			name: "null values",
			input: WorkflowJobResourceModel{
				ExtraVars:   customtypes.NewAAPCustomStringNull(),
				InventoryID: basetypes.NewInt64Null(),
				TemplateID:  types.Int64Value(1),
			},
			expected: []byte(`{}`),
		},
		{
			name: "extra vars only",
			input: WorkflowJobResourceModel{
				ExtraVars:   customtypes.NewAAPCustomStringValue("{\"test_name\":\"extra_vars\", \"provider\":\"aap\"}"),
				InventoryID: basetypes.NewInt64Null(),
			},
			expected: []byte(`{"extra_vars":"{\"test_name\":\"extra_vars\", \"provider\":\"aap\"}"}`),
		},
		{
			name: "inventory vars only",
			input: WorkflowJobResourceModel{
				ExtraVars:   customtypes.NewAAPCustomStringNull(),
				InventoryID: basetypes.NewInt64Value(201),
			},
			expected: []byte(`{"inventory": 201}`),
		},
		{
			name: "combined",
			input: WorkflowJobResourceModel{
				ExtraVars:   customtypes.NewAAPCustomStringValue("{\"test_name\":\"extra_vars\", \"provider\":\"aap\"}"),
				InventoryID: basetypes.NewInt64Value(3),
			},
			expected: []byte(`{"inventory":3,"extra_vars":"{\"test_name\":\"extra_vars\", \"provider\":\"aap\"}"}`),
		},
		{
			name: "manual_triggers",
			input: WorkflowJobResourceModel{
				Triggers:    types.MapNull(types.StringType),
				InventoryID: basetypes.NewInt64Value(3),
			},
			expected: []byte(`{"inventory": 3}`),
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

func TestWorkflowJobResourceParseHttpResponse(t *testing.T) {
	templateID := basetypes.NewInt64Value(1)
	inventoryID := basetypes.NewInt64Value(2)
	extraVars := customtypes.NewAAPCustomStringNull()
	jsonError := diag.Diagnostics{}
	jsonError.AddError("Error parsing JSON response from AAP", "invalid character 'N' looking for beginning of value")

	testTable := []struct {
		name     string
		input    []byte
		expected WorkflowJobResourceModel
		errors   diag.Diagnostics
	}{
		{
			name:     "JSON error",
			input:    []byte("Not valid JSON"),
			expected: WorkflowJobResourceModel{},
			errors:   jsonError,
		},
		{
			name:  "no ignored fields",
			input: []byte(`{"inventory":2,"workflow_job_template":1,"job_type": "run", "url": "/api/v2/workflow_jobs/14/", "status": "pending"}`),
			expected: WorkflowJobResourceModel{
				TemplateID:    templateID,
				Type:          types.StringValue("run"),
				URL:           types.StringValue("/api/v2/workflow_jobs/14/"),
				Status:        types.StringValue("pending"),
				InventoryID:   inventoryID,
				ExtraVars:     extraVars,
				IgnoredFields: types.ListNull(types.StringType),
			},
			errors: diag.Diagnostics{},
		},
		{
			name: "ignored fields",
			input: []byte(`{"inventory":2,"workflow_job_template":1,"job_type": "run", "url": "/api/v2/workflow_jobs/14/", "status":
			"pending", "ignored_fields": {"extra_vars": "{\"bucket_state\":\"absent\"}"}}`),
			expected: WorkflowJobResourceModel{
				TemplateID:    templateID,
				Type:          types.StringValue("run"),
				URL:           types.StringValue("/api/v2/workflow_jobs/14/"),
				Status:        types.StringValue("pending"),
				InventoryID:   inventoryID,
				ExtraVars:     extraVars,
				IgnoredFields: basetypes.NewListValueMust(types.StringType, []attr.Value{types.StringValue("extra_vars")}),
			},
			errors: diag.Diagnostics{},
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			resource := WorkflowJobResourceModel{}
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

func getWorkflowJobResourceFromStateFile(s *terraform.State) (map[string]interface{}, error) {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "aap_workflow_job" {
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

func testAccCheckWorkflowJobExists(s *terraform.State) error {
	_, err := getWorkflowJobResourceFromStateFile(s)
	return err
}

func testAccCheckWorkflowJobUpdate(urlBefore *string, shouldDiffer bool) func(s *terraform.State) error {
	return func(s *terraform.State) error {
		var jobURL string
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "aap_workflow_job" {
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

func testAccWorkflowJobResourcePreCheck(t *testing.T) {
	// ensure provider requirements
	testAccPreCheck(t)

	requiredAAPJobEnvVars := []string{
		"AAP_TEST_WORKFLOW_JOB_TEMPLATE_ID",
	}

	for _, key := range requiredAAPJobEnvVars {
		if v := os.Getenv(key); v == "" {
			t.Fatalf("'%s' environment variable must be set when running acceptance tests for job resource", key)
		}
	}
}

func TestAccAAPWorkflowJob_Basic(t *testing.T) {
	jobTemplateID := os.Getenv("AAP_TEST_WORKFLOW_JOB_TEMPLATE_ID")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccWorkflowJobResourcePreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccBasicWorkflowJob(jobTemplateID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestMatchResourceAttr("aap_workflow_job.test", "status", regexp.MustCompile("^(failed|pending|running|complete|successful|waiting)$")),
					resource.TestMatchResourceAttr("aap_workflow_job.test", "url", regexp.MustCompile("^/api(/controller)?/v2/workflow_jobs/[0-9]*/$")),
					testAccCheckWorkflowJobExists,
				),
			},
		},
	})
}

func TestAccAAPWorkflowJobWithNoInventoryID(t *testing.T) {
	jobTemplateID := os.Getenv("AAP_TEST_WORKFLOW_INVENTORY_ID")
	inventoryID := os.Getenv("AAP_TEST_INVENTORY_FOR_WF_ID")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccWorkflowJobResourcePreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccWorkflowJobWithNoInventoryID(jobTemplateID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestMatchResourceAttr("aap_workflow_job.wf_job", "status", regexp.MustCompile("^(failed|pending|running|complete|successful|waiting)$")),
					resource.TestMatchResourceAttr("aap_workflow_job.wf_job", "url", regexp.MustCompile("^/api(/controller)?/v2/workflow_jobs/[0-9]*/$")),
					resource.TestCheckResourceAttr("aap_workflow_job.wf_job", "inventory_id", inventoryID),
					resource.TestCheckResourceAttrWith("aap_workflow_job.wf_job", "inventory_id", func(value string) error {
						if value == "1" {
							return fmt.Errorf("inventory_id should not be 1, got %s", value)
						}
						return nil
					}),
					testAccCheckWorkflowJobExists,
					// assert that inventory id returned is not 1 and matches the new one.
				),
			},
		},
	})
}

func TestAccAAPWorkflowJob_UpdateWithSameParameters(t *testing.T) {
	var jobURLBefore string

	jobTemplateID := os.Getenv("AAP_TEST_WORKFLOW_JOB_TEMPLATE_ID")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccWorkflowJobResourcePreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccBasicWorkflowJob(jobTemplateID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestMatchResourceAttr("aap_workflow_job.test", "status", regexp.MustCompile("^(failed|pending|running|complete|successful|waiting)$")),
					resource.TestMatchResourceAttr("aap_workflow_job.test", "url", regexp.MustCompile("^/api(/controller)?/v2/workflow_jobs/[0-9]*/$")),
					testAccCheckWorkflowJobUpdate(&jobURLBefore, false),
				),
			},
			{
				Config: testAccBasicWorkflowJob(jobTemplateID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestMatchResourceAttr("aap_workflow_job.test", "status", regexp.MustCompile("^(failed|pending|running|complete|successful|waiting)$")),
					resource.TestMatchResourceAttr("aap_workflow_job.test", "url", regexp.MustCompile("^/api(/controller)?/v2/workflow_jobs/[0-9]*/$")),
					testAccCheckWorkflowJobUpdate(&jobURLBefore, false),
				),
			},
		},
	})
}

func TestAccAAPWorkflowJob_UpdateWithNewInventoryIdPromptOnLaunch(t *testing.T) {
	// In order to run the this test for the workflow job resource, you must have a working job template already in your AAP instance.
	// The job template used must be set to require an inventory on launch. Export the id of this job template into the
	// environment variable AAP_TEST_WORKFLOW_JOB_TEMPLATE_ID. Otherwise this test will fail when running the suite.

	var jobURLBefore string

	inventoryName := acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)
	jobTemplateID := os.Getenv("AAP_TEST_WORKFLOW_JOB_TEMPLATE_ID")
	ctx := context.Background()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccWorkflowJobResourcePreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccBasicWorkflowJob(jobTemplateID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestMatchResourceAttr("aap_workflow_job.test", "status", regexp.MustCompile("^(failed|pending|running|complete|successful|waiting)$")),
					resource.TestMatchResourceAttr("aap_workflow_job.test", "url", regexp.MustCompile("^/api(/controller)?/v2/workflow_jobs/[0-9]*/$")),
					testAccCheckWorkflowJobUpdate(&jobURLBefore, false),
				),
			},
			{
				Config: testAccUpdateWorkflowJobWithInventoryID(inventoryName, jobTemplateID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestMatchResourceAttr("aap_workflow_job.test", "status", regexp.MustCompile("^(failed|pending|running|complete|successful|waiting)$")),
					resource.TestMatchResourceAttr("aap_workflow_job.test", "url", regexp.MustCompile("^/api(/controller)?/v2/workflow_jobs/[0-9]*/$")),

					testAccCheckWorkflowJobUpdate(&jobURLBefore, true),
					// Wait for the job to finish so the inventory can be deleted
					testAccCheckWorkflowJobPause(ctx, "aap_workflow_job.test"),
				),
			},
		},
	})
}

func TestAccAAPWorkflowJob_UpdateWithTrigger(t *testing.T) {
	var jobURLBefore string

	jobTemplateID := os.Getenv("AAP_TEST_WORKFLOW_JOB_TEMPLATE_ID")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccWorkflowJobResourcePreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccBasicWorkflowJob(jobTemplateID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestMatchResourceAttr("aap_workflow_job.test", "status", regexp.MustCompile("^(failed|pending|running|complete|successful|waiting)$")),
					resource.TestMatchResourceAttr("aap_workflow_job.test", "url", regexp.MustCompile("^/api(/controller)?/v2/workflow_jobs/[0-9]*/$")),
					testAccCheckWorkflowJobUpdate(&jobURLBefore, false),
				),
			},
			{
				Config: testAccUpdateWorkflowJobWithTrigger(jobTemplateID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestMatchResourceAttr("aap_workflow_job.test", "status", regexp.MustCompile("^(failed|pending|running|complete|successful|waiting)$")),
					resource.TestMatchResourceAttr("aap_workflow_job.test", "url", regexp.MustCompile("^/api(/controller)?/v2/workflow_jobs/[0-9]*/$")),
					testAccCheckWorkflowJobUpdate(&jobURLBefore, true),
				),
			},
		},
	})
}

// testAccCheckWorkflowJobPause is designed to force the acceptance test framework to wait
// until a job is finished. This is needed when the associated inventory also must be
// deleted.
func testAccCheckWorkflowJobPause(ctx context.Context, name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		var apiModel WorkflowJobAPIModel
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
			err = json.Unmarshal(body, &apiModel)
			if err != nil {
				return retry.NonRetryableError(err)
			}
			if IsFinalStateAAPJob(apiModel.Status) {
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

func testAccBasicWorkflowJob(jobTemplateID string) string {
	return fmt.Sprintf(`
resource "aap_workflow_job" "test" {
	workflow_job_template_id   = %s
}
`, jobTemplateID)
}

func testAccWorkflowJobWithNoInventoryID(workflowJobTemplateID string) string {
	return fmt.Sprintf(`
resource "aap_workflow_job" "wf_job" {
	workflow_job_template_id = %s
	extra_vars = jsonencode({
    "foo": "bar"
	})
}
	`, workflowJobTemplateID)
}

func testAccUpdateWorkflowJobWithInventoryID(inventoryName, jobTemplateID string) string {
	return fmt.Sprintf(`
resource "aap_inventory" "test" {
  name = "%s"
}

resource "aap_workflow_job" "test" {
	workflow_job_template_id   = %s
	inventory_id = aap_inventory.test.id
}
`, inventoryName, jobTemplateID)
}

func testAccUpdateWorkflowJobWithTrigger(jobTemplateID string) string {
	return fmt.Sprintf(`
resource "aap_workflow_job" "test" {
	workflow_job_template_id   = %s
	triggers = {
		"key1" = "value1"
		"key2" = "value2"
	}
}
`, jobTemplateID)
}
