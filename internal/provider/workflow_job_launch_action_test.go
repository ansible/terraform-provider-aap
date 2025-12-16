package provider

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"math/big"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// Acceptance tests
func TestAccAAPWorkflowJobAction_Basic(t *testing.T) {
	jobTemplateID := os.Getenv("AAP_TEST_WORKFLOW_JOB_TEMPLATE_ID")
	randNum, _ := rand.Int(rand.Reader, big.NewInt(50000000))
	inventoryName := fmt.Sprintf("%s-%d", "tf-acc", randNum.Int64())

	// Capture stderr (where tflog is written)
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = io.Copy(&buf, r)
	}()

	// Set TF_LOG to DEBUG to capture the logs
	t.Setenv("TF_LOG", "DEBUG")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccWorkflowJobResourcePreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccBasicWorkflowJobAction(inventoryName, jobTemplateID),
			},
		},
	})

	// Restore stderr and get logs
	_ = w.Close()
	os.Stderr = old
	<-done

	// Verify logs contain expected content
	exists := false
	logs := buf.String()
	for _, logLine := range strings.Split(logs, "\n") {
		if strings.Contains(logLine, "workflow job launched") {
			if !strings.Contains(logLine, fmt.Sprintf("template_id=%s", jobTemplateID)) {
				t.Fatalf("expected log to contain template_id=%s, but got:\n%s", jobTemplateID, logLine)
			}
			exists = true
			break
		}
	}

	if !exists {
		t.Fatalf("expected job to be launched in logs, but received logs:\n%s", logs)
	}
}

func TestAccAAPWorkflowJobAction_fail(t *testing.T) {
	jobTemplateID := os.Getenv("AAP_TEST_WORKFLOW_JOB_TEMPLATE_FAIL_ID")
	randNum, _ := rand.Int(rand.Reader, big.NewInt(50000000))
	inventoryName := fmt.Sprintf("%s-%d", "tf-acc", randNum.Int64())

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccJobResourcePreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccBasicWorkflowJobAction(inventoryName, jobTemplateID),
				ExpectError: regexp.MustCompile(".*AAP workflow job failed.*"),
			},
		},
	})
}

func TestAccAAPWorkflowJobAction_failIgnore(t *testing.T) {
	jobTemplateID := os.Getenv("AAP_TEST_WORKFLOW_JOB_TEMPLATE_FAIL_ID")
	randNum, _ := rand.Int(rand.Reader, big.NewInt(50000000))
	inventoryName := fmt.Sprintf("%s-%d", "tf-acc", randNum.Int64())

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccJobResourcePreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccBasicWorkflowJobActionIgnoreFail(inventoryName, jobTemplateID),
			},
		},
	})
}

func testAccBasicWorkflowJobAction(inventoryName, jobTemplateID string) string {
	return fmt.Sprintf(`
resource "aap_inventory" "test" {
	name = "%s"
	lifecycle {
		action_trigger {
			events = [after_create]
			actions = [action.aap_workflow_job_launch.test]
		}
	}
}

action "aap_workflow_job_launch" "test" {
	config {
		workflow_job_template_id = %s
		inventory_id = aap_inventory.test.id
		wait_for_completion 	 = true
	}
}
`, inventoryName, jobTemplateID)
}

func testAccBasicWorkflowJobActionIgnoreFail(inventoryName, jobTemplateID string) string {
	return fmt.Sprintf(`
resource "aap_inventory" "test" {
	name = "%s"
	lifecycle {
		action_trigger {
			events = [after_create]
			actions = [action.aap_workflow_job_launch.test]
		}
	}
}

action "aap_workflow_job_launch" "test" {
	config {
		workflow_job_template_id = %s
		inventory_id = aap_inventory.test.id
		wait_for_completion 	 = true
		ignore_job_results 	 	 = true
	}
}
`, inventoryName, jobTemplateID)
}

// TestAccAAPWorkflowJobAction_AllFieldsOnPrompt tests that a workflow job action with all fields on prompt
// can be launched successfully when all required fields are provided.
func TestAccAAPWorkflowJobAction_AllFieldsOnPrompt(t *testing.T) {
	workflowJobTemplateID := os.Getenv("AAP_TEST_WORKFLOW_JOB_TEMPLATE_ALL_FIELDS_PROMPT_ID")
	if workflowJobTemplateID == "" {
		t.Skip("AAP_TEST_WORKFLOW_JOB_TEMPLATE_ALL_FIELDS_PROMPT_ID environment variable not set")
	}
	labelID := os.Getenv("AAP_TEST_LABEL_ID")
	if labelID == "" {
		t.Skip("AAP_TEST_LABEL_ID environment variable not set")
	}

	randNum, _ := rand.Int(rand.Reader, big.NewInt(50000000))
	inventoryName := fmt.Sprintf("%s-%d", "tf-acc", randNum.Int64())

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccWorkflowJobResourcePreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccWorkflowJobActionAllFieldsOnPrompt(inventoryName, workflowJobTemplateID, labelID),
			},
		},
	})
}

// TestAccAAPWorkflowJobAction_AllFieldsOnPrompt_MissingRequired tests that a workflow job action with all
// fields on prompt fails when required fields are not provided.
func TestAccAAPWorkflowJobAction_AllFieldsOnPrompt_MissingRequired(t *testing.T) {
	workflowJobTemplateID := os.Getenv("AAP_TEST_WORKFLOW_JOB_TEMPLATE_ALL_FIELDS_PROMPT_ID")
	if workflowJobTemplateID == "" {
		t.Skip("AAP_TEST_WORKFLOW_JOB_TEMPLATE_ALL_FIELDS_PROMPT_ID environment variable not set")
	}
	randNum, _ := rand.Int(rand.Reader, big.NewInt(50000000))
	inventoryName := fmt.Sprintf("%s-%d", "tf-acc", randNum.Int64())

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccWorkflowJobResourcePreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccWorkflowJobActionAllFieldsOnPromptMissingRequired(inventoryName, workflowJobTemplateID),
				ExpectError: regexp.MustCompile(".*Missing required field.*"),
			},
		},
	})
}

func testAccWorkflowJobActionAllFieldsOnPrompt(inventoryName, workflowJobTemplateID, labelID string) string {
	return fmt.Sprintf(`
resource "aap_inventory" "test" {
	name = "%s"
	lifecycle {
		action_trigger {
			events = [after_create]
			actions = [action.aap_workflow_job_launch.test]
		}
	}
}

action "aap_workflow_job_launch" "test" {
	config {
		workflow_job_template_id = %s
		inventory_id             = aap_inventory.test.id
		extra_vars               = "{\"test_var\": \"test_value\"}"
		limit                    = "localhost"
		job_tags                 = "test"
		skip_tags                = "skip"
		labels                   = [%s]
		wait_for_completion      = true
	}
}
`, inventoryName, workflowJobTemplateID, labelID)
}

func testAccWorkflowJobActionAllFieldsOnPromptMissingRequired(inventoryName, workflowJobTemplateID string) string {
	return fmt.Sprintf(`
resource "aap_inventory" "test" {
	name = "%s"
	lifecycle {
		action_trigger {
			events = [after_create]
			actions = [action.aap_workflow_job_launch.test]
		}
	}
}

action "aap_workflow_job_launch" "test" {
	config {
		workflow_job_template_id = %s
		wait_for_completion      = true
	}
}
`, inventoryName, workflowJobTemplateID)
}
