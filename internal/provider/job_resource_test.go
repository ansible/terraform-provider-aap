package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestParseHttpResponse(t *testing.T) {
	templateID := basetypes.NewInt64Value(1)
	inventoryID := basetypes.NewInt64Value(2)
	extraVars := jsontypes.NewNormalizedNull()
	testTable := []struct {
		name     string
		body     []byte
		expected jobResourceModel
		failure  bool
	}{
		{
			name:    "no ignored fields",
			failure: false,
			body:    []byte(`{"job_type": "run", "url": "/api/v2/jobs/14/", "status": "pending"}`),
			expected: jobResourceModel{
				TemplateID:    templateID,
				Type:          types.StringValue("run"),
				URL:           types.StringValue("/api/v2/jobs/14/"),
				Status:        types.StringValue("pending"),
				InventoryID:   inventoryID,
				ExtraVars:     extraVars,
				IgnoredFields: types.ListNull(types.StringType),
			},
		},
		{
			name:    "ignored fields",
			failure: false,
			body: []byte(`{"job_type": "run", "url": "/api/v2/jobs/14/", "status":
			"pending", "ignored_fields": {"extra_vars": "{\"bucket_state\":\"absent\"}"}}`),
			expected: jobResourceModel{
				TemplateID:    templateID,
				Type:          types.StringValue("run"),
				URL:           types.StringValue("/api/v2/jobs/14/"),
				Status:        types.StringValue("pending"),
				InventoryID:   inventoryID,
				ExtraVars:     extraVars,
				IgnoredFields: basetypes.NewListValueMust(types.StringType, []attr.Value{types.StringValue("extra_vars")}),
			},
		},
		{
			name:     "bad json",
			failure:  true,
			body:     []byte(`{job_type: run}`),
			expected: jobResourceModel{},
		},
	}
	for _, tc := range testTable {
		t.Run(tc.name, func(t *testing.T) {
			d := jobResourceModel{
				TemplateID:  templateID,
				InventoryID: inventoryID,
				ExtraVars:   extraVars,
			}
			err := d.ParseHTTPResponse(tc.body)
			if tc.failure {
				if err == nil {
					t.Errorf("expecting failure while the process has not failed")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected process failure (%s)", err.Error())
				} else if !reflect.DeepEqual(tc.expected, d) {
					t.Errorf("expected (%v) - result (%v)", tc.expected, d)
				}
			}
		})
	}
}

func TestCreateRequestBody(t *testing.T) {
	testTable := []struct {
		name     string
		input    jobResourceModel
		expected []byte
	}{
		{
			name: "unknown fields",
			input: jobResourceModel{
				ExtraVars:   jsontypes.NewNormalizedNull(),
				InventoryID: basetypes.NewInt64Unknown(),
			},
			expected: nil,
		},
		{
			name: "null fields",
			input: jobResourceModel{
				ExtraVars:   jsontypes.NewNormalizedNull(),
				InventoryID: basetypes.NewInt64Null(),
			},
			expected: nil,
		},
		{
			name: "extra vars only",
			input: jobResourceModel{
				ExtraVars:   jsontypes.NewNormalizedValue("{\"test_name\":\"extra_vars\", \"provider\":\"aap\"}"),
				InventoryID: basetypes.NewInt64Null(),
			},
			expected: []byte(`{"extra_vars":{"test_name":"extra_vars","provider":"aap"}}`),
		},
		{
			name: "inventory vars only",
			input: jobResourceModel{
				ExtraVars:   jsontypes.NewNormalizedNull(),
				InventoryID: basetypes.NewInt64Value(201),
			},
			expected: []byte(`{"inventory": 201}`),
		},
		{
			name: "combined",
			input: jobResourceModel{
				ExtraVars:   jsontypes.NewNormalizedValue("{\"test_name\":\"extra_vars\", \"provider\":\"aap\"}"),
				InventoryID: basetypes.NewInt64Value(3),
			},
			expected: []byte(`{"inventory": 3, "extra_vars":{"test_name":"extra_vars","provider":"aap"}}`),
		},
		{
			name: "manual_triggers",
			input: jobResourceModel{
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
					t.Fatal("expected nil but result is not nil")
				}
				if tc.expected != nil && computed == nil {
					t.Fatal("expected result not nil but result is nil")
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

type MockJobResource struct {
	ID        string
	URL       string
	Inventory string
	Response  map[string]string
}

func NewMockJobResource(id, inventory, url string) *MockJobResource {
	return &MockJobResource{
		ID:        id,
		URL:       url,
		Inventory: inventory,
		Response:  map[string]string{},
	}
}

func (d *MockJobResource) GetTemplateID() string {
	return d.ID
}

func (d *MockJobResource) GetURL() string {
	return d.URL
}

func (d *MockJobResource) ParseHTTPResponse(body []byte) error {
	err := json.Unmarshal(body, &d.Response)
	if err != nil {
		return err
	}
	return nil
}

func (d *MockJobResource) CreateRequestBody() ([]byte, diag.Diagnostics) {
	var diags diag.Diagnostics
	if len(d.Inventory) == 0 {
		return nil, diags
	}
	m := map[string]string{"Inventory": d.Inventory}
	jsonRaw, err := json.Marshal(m)
	if err != nil {
		diags.AddError("Json Marshall Error", err.Error())
		return nil, diags
	}
	return jsonRaw, diags
}

func TestCreateJob(t *testing.T) {
	testTable := []struct {
		name          string
		ID            string
		Inventory     string
		expected      map[string]string
		acceptMethods []string
		httpCode      int
		failed        bool
	}{
		{
			name:          "create job simple job (no request data)",
			ID:            "1",
			Inventory:     "",
			httpCode:      http.StatusCreated,
			failed:        false,
			acceptMethods: []string{"POST", "post"},
			expected:      JobResponse1,
		},
		{
			name:          "create job with request data",
			ID:            "1",
			Inventory:     "3",
			httpCode:      http.StatusCreated,
			failed:        false,
			acceptMethods: []string{"POST", "post"},
			expected:      mergeStringMaps(JobResponse1, map[string]string{"Inventory": "3"}),
		},
		{
			name:          "try with non existing template id",
			ID:            "-1",
			Inventory:     "3",
			httpCode:      http.StatusCreated,
			failed:        true,
			acceptMethods: []string{"POST", "post"},
			expected:      nil,
		},
		{
			name:          "Unexpected method leading to not found",
			ID:            "1",
			Inventory:     "3",
			httpCode:      http.StatusCreated,
			failed:        true,
			acceptMethods: []string{"GET", "get"},
			expected:      nil,
		},
		{
			name:          "using another template id",
			ID:            "2",
			Inventory:     "1",
			httpCode:      http.StatusCreated,
			failed:        false,
			acceptMethods: []string{"POST", "post"},
			expected:      mergeStringMaps(JobResponse2, map[string]string{"Inventory": "1"}),
		},
	}

	for _, tc := range testTable {
		t.Run(tc.name, func(t *testing.T) {
			resource := NewMockJobResource(tc.ID, tc.Inventory, "")

			job := JobResource{
				client: NewMockHTTPClient(tc.acceptMethods, tc.httpCode),
			}
			diags := job.CreateJob(resource)
			if (tc.failed && !diags.HasError()) || (!tc.failed && diags.HasError()) {
				if diags.HasError() {
					t.Errorf("process has failed while it should not")
					for _, d := range diags {
						t.Errorf("Summary = '%s' - details = '%s'", d.Summary(), d.Detail())
					}
				} else {
					t.Errorf("failure expected but the process did not failed!!")
				}
			} else if !tc.failed && !reflect.DeepEqual(tc.expected, resource.Response) {
				t.Errorf("expected (%v)", tc.expected)
				t.Errorf("computed (%v)", resource.Response)
			}
		})
	}
}

func TestReadJob(t *testing.T) {
	testTable := []struct {
		name          string
		url           string
		expected      map[string]string
		acceptMethods []string
		httpCode      int
		failed        bool
	}{
		{
			name:          "Read existing job",
			url:           "/api/v2/jobs/1/",
			httpCode:      http.StatusOK,
			failed:        false,
			acceptMethods: []string{"GET", "get"},
			expected:      JobResponse1,
		},
		{
			name:          "Read another job",
			url:           "/api/v2/jobs/2/",
			httpCode:      http.StatusOK,
			failed:        false,
			acceptMethods: []string{"GET", "get"},
			expected:      JobResponse3,
		},
		{
			name:          "GET not part of accepted methods",
			url:           "/api/v2/jobs/2/",
			httpCode:      http.StatusOK,
			failed:        true,
			acceptMethods: []string{"HEAD"},
			expected:      nil,
		},
		{
			name:          "no url provided",
			url:           "",
			httpCode:      http.StatusOK,
			failed:        false,
			acceptMethods: []string{"GET", "get"},
			expected:      map[string]string{},
		},
	}

	for _, tc := range testTable {
		t.Run(tc.name, func(t *testing.T) {
			resource := NewMockJobResource("", "", tc.url)

			job := JobResource{
				client: NewMockHTTPClient(tc.acceptMethods, tc.httpCode),
			}
			err := job.ReadJob(resource)
			if (tc.failed && err == nil) || (!tc.failed && err != nil) {
				if err != nil {
					t.Errorf("process has failed with (%s) while it should not", err.Error())
				} else {
					t.Errorf("failure expected but the process did not failed!!")
				}
			} else if !tc.failed && !reflect.DeepEqual(tc.expected, resource.Response) {
				t.Errorf("expected (%v)", tc.expected)
				t.Errorf("computed (%v)", resource.Response)
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
		jobURL := rs.Primary.Attributes["job_url"]
		return testGetResource(jobURL)
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
			jobURL = rs.Primary.Attributes["job_url"]
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
		"AAP_TEST_JOB_INVENTORY_ID",
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
					resource.TestMatchResourceAttr(resourceName, "job_url", regexp.MustCompile("^/api/v2/jobs/[0-9]*/$")),
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
					resource.TestMatchResourceAttr(resourceName, "job_url", regexp.MustCompile("^/api/v2/jobs/[0-9]*/$")),
					testAccCheckJobUpdate(&jobURLBefore, false),
				),
			},
			{
				Config: testAccBasicJob(jobTemplateID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestMatchResourceAttr(resourceName, "status", regexp.MustCompile("^(failed|pending|running|complete|successful|waiting)$")),
					resource.TestMatchResourceAttr(resourceName, "job_type", regexp.MustCompile("^(run|check)$")),
					resource.TestMatchResourceAttr(resourceName, "job_url", regexp.MustCompile("^/api/v2/jobs/[0-9]*/$")),
					testAccCheckJobUpdate(&jobURLBefore, false),
				),
			},
		},
	})
}

func TestAccAAPJob_UpdateWithNewInventoryId(t *testing.T) {
	var jobURLBefore string

	jobTemplateID := os.Getenv("AAP_TEST_JOB_TEMPLATE_ID")
	inventoryID := os.Getenv("AAP_TEST_JOB_INVENTORY_ID")

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
					resource.TestMatchResourceAttr(resourceName, "job_url", regexp.MustCompile("^/api/v2/jobs/[0-9]*/$")),
					testAccCheckJobUpdate(&jobURLBefore, false),
				),
			},
			{
				Config: testAccUpdateJobWithInventoryID(jobTemplateID, inventoryID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestMatchResourceAttr(resourceName, "status", regexp.MustCompile("^(failed|pending|running|complete|successful|waiting)$")),
					resource.TestMatchResourceAttr(resourceName, "job_type", regexp.MustCompile("^(run|check)$")),
					resource.TestMatchResourceAttr(resourceName, "job_url", regexp.MustCompile("^/api/v2/jobs/[0-9]*/$")),
					testAccCheckJobUpdate(&jobURLBefore, true),
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
					resource.TestMatchResourceAttr(resourceName, "job_url", regexp.MustCompile("^/api/v2/jobs/[0-9]*/$")),
					testAccCheckJobUpdate(&jobURLBefore, false),
				),
			},
			{
				Config: testAccUpdateJobWithTrigger(jobTemplateID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestMatchResourceAttr(resourceName, "status", regexp.MustCompile("^(failed|pending|running|complete|successful|waiting)$")),
					resource.TestMatchResourceAttr(resourceName, "job_type", regexp.MustCompile("^(run|check)$")),
					resource.TestMatchResourceAttr(resourceName, "job_url", regexp.MustCompile("^/api/v2/jobs/[0-9]*/$")),
					testAccCheckJobUpdate(&jobURLBefore, true),
				),
			},
		},
	})
}

func testAccBasicJob(jobTemplateID string) string {
	return fmt.Sprintf(`
resource "aap_job" "test" {
	job_template_id   = %s
}
`, jobTemplateID)
}

func testAccUpdateJobWithInventoryID(jobTemplateID, inventoryID string) string {
	return fmt.Sprintf(`
resource "aap_job" "test" {
	job_template_id   = %s
	inventory_id = %s
}
`, jobTemplateID, inventoryID)
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
