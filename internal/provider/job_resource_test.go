package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"slices"
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

func TestJobResourceSchema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
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

func TestJobResourceCreateRequestBody(t *testing.T) {
	var testTable = []struct {
		name     string
		input    JobResourceModel
		expected []byte
	}{
		{
			name: "unknown values",
			input: JobResourceModel{
				ExtraVars:   customtypes.NewAAPCustomStringUnknown(),
				InventoryID: basetypes.NewInt64Unknown(),
				TemplateID:  types.Int64Value(1),
			},
			expected: []byte(`{"inventory":1}`),
		},
		{
			name: "null values",
			input: JobResourceModel{
				ExtraVars:   customtypes.NewAAPCustomStringNull(),
				InventoryID: basetypes.NewInt64Null(),
				TemplateID:  types.Int64Value(1),
			},
			expected: []byte(`{"inventory":1}`),
		},
		{
			name: "extra vars only",
			input: JobResourceModel{
				ExtraVars:   customtypes.NewAAPCustomStringValue("{\"test_name\":\"extra_vars\", \"provider\":\"aap\"}"),
				InventoryID: basetypes.NewInt64Null(),
			},
			expected: []byte(`{"inventory":1,"extra_vars":"{\"test_name\":\"extra_vars\", \"provider\":\"aap\"}"}`),
		},
		{
			name: "inventory vars only",
			input: JobResourceModel{
				ExtraVars:   customtypes.NewAAPCustomStringNull(),
				InventoryID: basetypes.NewInt64Value(201),
			},
			expected: []byte(`{"inventory": 201}`),
		},
		{
			name: "combined",
			input: JobResourceModel{
				ExtraVars:   customtypes.NewAAPCustomStringValue("{\"test_name\":\"extra_vars\", \"provider\":\"aap\"}"),
				InventoryID: basetypes.NewInt64Value(3),
			},
			expected: []byte(`{"inventory":3,"extra_vars":"{\"test_name\":\"extra_vars\", \"provider\":\"aap\"}"}`),
		},
		{
			name: "manual_triggers",
			input: JobResourceModel{
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

func TestJobResourceParseHttpResponse(t *testing.T) {
	templateID := basetypes.NewInt64Value(1)
	inventoryID := basetypes.NewInt64Value(2)
	extraVars := customtypes.NewAAPCustomStringNull()
	jsonError := diag.Diagnostics{}
	jsonError.AddError("Error parsing JSON response from AAP", "invalid character 'N' looking for beginning of value")

	var testTable = []struct {
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
			expected: JobResourceModel{
				TemplateID:    templateID,
				Type:          types.StringValue("run"),
				URL:           types.StringValue("/api/v2/jobs/14/"),
				Status:        types.StringValue("pending"),
				InventoryID:   inventoryID,
				ExtraVars:     extraVars,
				IgnoredFields: types.ListNull(types.StringType),
			},
			errors: diag.Diagnostics{},
		},
		{
			name: "ignored fields",
			input: []byte(`{"inventory":2,"job_template":1,"job_type": "run", "url": "/api/v2/jobs/14/", "status":
			"pending", "ignored_fields": {"extra_vars": "{\"bucket_state\":\"absent\"}"}}`),
			expected: JobResourceModel{
				TemplateID:    templateID,
				Type:          types.StringValue("run"),
				URL:           types.StringValue("/api/v2/jobs/14/"),
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
	}

	for _, key := range requiredAAPJobEnvVars {
		if v := os.Getenv(key); v == "" {
			t.Fatalf("'%s' environment variable must be set when running acceptance tests for job resource", key)
		}
	}
}

const resourceName = "aap_job.test"

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
					resource.TestMatchResourceAttr(resourceName, "status", regexp.MustCompile("^(failed|pending|running|complete|successful|waiting)$")),
					resource.TestMatchResourceAttr(resourceName, "job_type", regexp.MustCompile("^(run|check)$")),
					resource.TestMatchResourceAttr(resourceName, "url", regexp.MustCompile("^/api/v2/jobs/[0-9]*/$")),
					testAccCheckJobExists,
				),
			},
		},
	})
}

//nolint:dupl
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
					resource.TestMatchResourceAttr(resourceName, "status", regexp.MustCompile("^(failed|pending|running|complete|successful|waiting)$")),
					resource.TestMatchResourceAttr(resourceName, "job_type", regexp.MustCompile("^(run|check)$")),
					resource.TestMatchResourceAttr(resourceName, "url", regexp.MustCompile("^/api/v2/jobs/[0-9]*/$")),
					testAccCheckJobUpdate(&jobURLBefore, false),
				),
			},
			{
				Config: testAccBasicJob(jobTemplateID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestMatchResourceAttr(resourceName, "status", regexp.MustCompile("^(failed|pending|running|complete|successful|waiting)$")),
					resource.TestMatchResourceAttr(resourceName, "job_type", regexp.MustCompile("^(run|check)$")),
					resource.TestMatchResourceAttr(resourceName, "url", regexp.MustCompile("^/api/v2/jobs/[0-9]*/$")),
					testAccCheckJobUpdate(&jobURLBefore, false),
				),
			},
		},
	})
}

func TestAccAAPJob_UpdateWithNewInventoryId(t *testing.T) {
	var jobURLBefore string

	inventoryName := acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)
	jobTemplateID := os.Getenv("AAP_TEST_JOB_TEMPLATE_ID")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccJobResourcePreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccBasicJob(jobTemplateID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestMatchResourceAttr(resourceName, "status", regexp.MustCompile("^(failed|pending|running|complete|successful|waiting)$")),
					resource.TestMatchResourceAttr(resourceName, "job_type", regexp.MustCompile("^(run|check)$")),
					resource.TestMatchResourceAttr(resourceName, "url", regexp.MustCompile("^/api/v2/jobs/[0-9]*/$")),
					testAccCheckJobUpdate(&jobURLBefore, false),
				),
			},
			{
				Config: testAccUpdateJobWithInventoryID(inventoryName, jobTemplateID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestMatchResourceAttr(resourceName, "status", regexp.MustCompile("^(failed|pending|running|complete|successful|waiting)$")),
					resource.TestMatchResourceAttr(resourceName, "job_type", regexp.MustCompile("^(run|check)$")),
					resource.TestMatchResourceAttr(resourceName, "url", regexp.MustCompile("^/api/v2/jobs/[0-9]*/$")),
					testAccCheckJobUpdate(&jobURLBefore, true),
					// Wait for the job to finish so the inventory can be deleted
					testAccCheckJobPause("aap_job.test"),
				),
			},
		},
	})
}

//nolint:dupl
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
					resource.TestMatchResourceAttr(resourceName, "status", regexp.MustCompile("^(failed|pending|running|complete|successful|waiting)$")),
					resource.TestMatchResourceAttr(resourceName, "job_type", regexp.MustCompile("^(run|check)$")),
					resource.TestMatchResourceAttr(resourceName, "url", regexp.MustCompile("^/api/v2/jobs/[0-9]*/$")),
					testAccCheckJobUpdate(&jobURLBefore, false),
				),
			},
			{
				Config: testAccUpdateJobWithTrigger(jobTemplateID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestMatchResourceAttr(resourceName, "status", regexp.MustCompile("^(failed|pending|running|complete|successful|waiting)$")),
					resource.TestMatchResourceAttr(resourceName, "job_type", regexp.MustCompile("^(run|check)$")),
					resource.TestMatchResourceAttr(resourceName, "url", regexp.MustCompile("^/api/v2/jobs/[0-9]*/$")),
					testAccCheckJobUpdate(&jobURLBefore, true),
				),
			},
		},
	})
}

// testAccCheckJobPause is just a slightly more intelligent sleep function
// designed to force the acceptance test framework to wait until a job is
// finished. This is needed when the associated inventory also must be
// deleted.
func testAccCheckJobPause(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		var jobApiModel JobAPIModel
		statuses := []string{"failed", "complete", "successful"}
		job, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("job (%s) not found in state", name)
		}
		i := 0
		for i < 10 {
			i += 1
			body, err := testGetResource(job.Primary.Attributes["url"])
			if err != nil {
				return err
			}
			err = json.Unmarshal(body, &jobApiModel)
			if err != nil {
				return err
			}
			if slices.Contains(statuses, jobApiModel.Status) {
				break
			}
			time.Sleep(6 * time.Second)
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
