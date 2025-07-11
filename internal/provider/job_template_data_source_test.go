package provider

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/ansible/terraform-provider-aap/internal/provider/customtypes"
	fwdatasource "github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	tftypes "github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestJobTemplateDataSourceSchema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schemaRequest := fwdatasource.SchemaRequest{}
	schemaResponse := &fwdatasource.SchemaResponse{}

	// Instantiate the JobTemplateDataSource and call its Schema method
	NewJobTemplateDataSource().Schema(ctx, schemaRequest, schemaResponse)

	if schemaResponse.Diagnostics.HasError() {
		t.Fatalf("Schema method diagnostics: %+v", schemaResponse.Diagnostics)
	}

	// Validate the schema
	diagnostics := schemaResponse.Schema.ValidateImplementation(ctx)

	if diagnostics.HasError() {
		t.Fatalf("Schema validation diagnostics: %+v", diagnostics)
	}
}

func TestJobTemplateDataSourceParseHttpResponse(t *testing.T) {
	jsonError := diag.Diagnostics{}
	jsonError.AddError("Error parsing JSON response from AAP", "invalid character 'N' looking for beginning of value")

	var testTable = []struct {
		name     string
		input    []byte
		expected JobTemplateDataSourceModel
		errors   diag.Diagnostics
	}{
		{
			name:     "JSON error",
			input:    []byte("Not valid JSON"),
			expected: JobTemplateDataSourceModel{},
			errors:   jsonError,
		},
		{
			name:  "missing values",
			input: []byte(`{"id":1,"organization":2,"url":"/job_templates/1/"}`),
			expected: JobTemplateDataSourceModel{
				BaseDetailSourceModelWithOrg: BaseDetailSourceModelWithOrg{
					BaseDetailSourceModel: BaseDetailSourceModel{
						Id:  tftypes.Int64Value(1),
						URL: tftypes.StringValue("/job_templates/1/"),

						Description: tftypes.StringNull(),
						Name:        tftypes.StringNull(),
						NamedUrl:    tftypes.StringNull(),
						Variables:   customtypes.NewAAPCustomStringNull(),
					},
					Organization:     tftypes.Int64Value(2),
					OrganizationName: tftypes.StringNull(),
				},
			},
			errors: diag.Diagnostics{},
		},
		{
			name: "all values",
			input: []byte(
				`{"id":1,"organization":2,"url":"/job_templates/1/","name":"my job template","description":"My Test Job Template","variables":"{\"foo\":\"bar\"}"}`,
			),
			expected: JobTemplateDataSourceModel{
				BaseDetailSourceModelWithOrg: BaseDetailSourceModelWithOrg{
					BaseDetailSourceModel: BaseDetailSourceModel{
						Id:          tftypes.Int64Value(1),
						URL:         tftypes.StringValue("/job_templates/1/"),
						Description: tftypes.StringValue("My Test Job Template"),
						Name:        tftypes.StringValue("my job template"),
						NamedUrl:    tftypes.StringNull(),
						Variables:   customtypes.NewAAPCustomStringValue("{\"foo\":\"bar\"}"),
					},
					Organization:     tftypes.Int64Value(2),
					OrganizationName: tftypes.StringNull(),
				},
			},
			errors: diag.Diagnostics{},
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			source := JobTemplateDataSourceModel{}
			diags := source.ParseHttpResponse(test.input)
			if !test.errors.Equal(diags) {
				t.Errorf("Expected error diagnostics (%s), Received (%s)", test.errors, diags)
			}
			if test.expected != source {
				t.Errorf("Expected (%s) not equal to actual (%s)", test.expected, source)
			}
		})
	}
}

func TestAccJobTemplateDataSource(t *testing.T) {
	jobTemplateID := os.Getenv("AAP_TEST_JOB_TEMPLATE_ID")
	jobTemplateName := "Demo Job Template"
	jobTemplateOrg := "Default"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read
			{
				Config: testAccJobTemplateDataSourceFromId(jobTemplateID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.aap_job_template.test", "name"),
					resource.TestCheckResourceAttrSet("data.aap_job_template.test", "organization"),
					resource.TestCheckResourceAttrSet("data.aap_job_template.test", "url"),
				),
			},
			// Read
			{
				Config: testAccJobTemplateDataSourceFromNamedUrl(jobTemplateName, jobTemplateOrg),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.aap_job_template.test", "name"),
					resource.TestCheckResourceAttrSet("data.aap_job_template.test", "organization"),
					resource.TestCheckResourceAttrSet("data.aap_job_template.test", "url"),
				),
			},
			// Read
			{
				Config: testAccJobTemplateDataSourceVariable(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.aap_job_template.test", "name"),
					resource.TestCheckResourceAttrSet("data.aap_job_template.test", "organization"),
					resource.TestCheckResourceAttrSet("data.aap_job_template.test", "url"),
				),
			},
		},
		CheckDestroy: testAccCheckInventoryResourceDestroy,
	})
}

// testAccJobTemplateDataSourceFromId configures the Job Template Data Source for testing
func testAccJobTemplateDataSourceFromId(id string) string {
	return fmt.Sprintf(`
data "aap_job_template" "test" {
  id = %s
}
`, id)
}

func testAccJobTemplateDataSourceFromNamedUrl(name string, orgName string) string {
	return fmt.Sprintf(`
data "aap_job_template" "test" {
  name = "%s"
  organization_name = "%s"
}
`, name, orgName)
}

func testAccJobTemplateDataSourceVariable() string {
	return `
variable "job_template_name" {
  description = "Name of the AAP job template to run"
  type        = string
  default     = "Demo Job Template"
}

data "aap_job_template" "test" {
  name = var.job_template_name
  organization_name = "Default"
}`
}
