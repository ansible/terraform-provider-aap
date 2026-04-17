package provider

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"go.uber.org/mock/gomock"
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

type workflowJobActionTestCase struct {
	name              string
	waitForCompletion bool
	ignoreJobResults  bool
	finalStatus       string
	expectError       bool
	expectWarning     bool
	launchFails       bool
	invalidJSON       bool
}

func setupWorkflowJobActionMocks(mockClient *MockProviderHTTPClient, tc workflowJobActionTestCase) {
	// Mock GetLaunchWorkflowJob
	mockClient.EXPECT().getAPIEndpoint().Return("/api/v2").Times(1)
	mockClient.EXPECT().
		doRequest(http.MethodGet, gomock.Any(), nil, nil).
		Return(&http.Response{StatusCode: http.StatusOK}, []byte(`{"ask_variables_on_launch": false}`), nil).
		Times(1)

	// Mock LaunchJobTemplate
	mockClient.EXPECT().getAPIEndpoint().Return("/api/v2").Times(1)
	if tc.launchFails {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v2/workflow_job_templates/123/launch", nil)
		mockClient.EXPECT().
			doRequest(http.MethodPost, gomock.Any(), nil, gomock.Any()).
			Return(&http.Response{StatusCode: http.StatusBadRequest, Request: req}, []byte(`{"error":"bad request"}`), nil).
			Times(1)
	} else {
		var launchResp []byte
		if tc.invalidJSON {
			launchResp = []byte(`{invalid json`)
		} else {
			launchResp = []byte(`{
				"workflow_job_template": 123,
				"url": "/api/v2/workflow_jobs/456/",
				"status": "pending",
				"inventory": 10
			}`)
		}
		mockClient.EXPECT().
			doRequest(http.MethodPost, gomock.Any(), nil, gomock.Any()).
			Return(&http.Response{StatusCode: http.StatusCreated}, launchResp, nil).
			Times(1)
	}

	// Mock wait-for-completion if enabled
	if tc.waitForCompletion && !tc.launchFails && !tc.invalidJSON {
		mockClient.EXPECT().
			Get(gomock.Any()).
			Return([]byte(fmt.Sprintf(`{"status":"%s"}`, tc.finalStatus)), nil).
			Times(1)
	}
}

func createWorkflowJobActionRequest(t *testing.T, workflowAction *WorkflowJobAction, tc workflowJobActionTestCase) action.InvokeRequest {
	ctx := t.Context()
	schemaReq := action.SchemaRequest{}
	schemaResp := &action.SchemaResponse{}
	workflowAction.Schema(ctx, schemaReq, schemaResp)

	configType := schemaResp.Schema.Type().TerraformType(ctx)
	configVal := tftypes.NewValue(configType, map[string]tftypes.Value{
		"workflow_job_template_id":            tftypes.NewValue(tftypes.Number, big.NewFloat(123)),
		"inventory_id":                        tftypes.NewValue(tftypes.Number, big.NewFloat(10)),
		"wait_for_completion":                 tftypes.NewValue(tftypes.Bool, tc.waitForCompletion),
		"wait_for_completion_timeout_seconds": tftypes.NewValue(tftypes.Number, big.NewFloat(120)),
		"ignore_job_results":                  tftypes.NewValue(tftypes.Bool, tc.ignoreJobResults),
		"extra_vars":                          tftypes.NewValue(tftypes.String, nil),
		"limit":                               tftypes.NewValue(tftypes.String, nil),
		"job_tags":                            tftypes.NewValue(tftypes.String, nil),
		"skip_tags":                           tftypes.NewValue(tftypes.String, nil),
		"labels":                              tftypes.NewValue(tftypes.List{ElementType: tftypes.Number}, nil),
	})

	return action.InvokeRequest{
		Config: tfsdk.Config{
			Raw:    configVal,
			Schema: schemaResp.Schema,
		},
	}
}

// Unit tests for Invoke method
func TestWorkflowJobActionInvoke(t *testing.T) {
	t.Parallel()

	testCases := []workflowJobActionTestCase{
		{
			name:              "fire and forget - successful launch",
			waitForCompletion: false,
			expectError:       false,
		},
		{
			name:              "wait for completion - job succeeds",
			waitForCompletion: true,
			finalStatus:       "successful",
			expectError:       false,
		},
		{
			name:              "wait for completion - job fails",
			waitForCompletion: true,
			finalStatus:       "failed",
			expectError:       true,
		},
		{
			name:              "wait for completion - job fails but ignored",
			waitForCompletion: true,
			ignoreJobResults:  true,
			finalStatus:       "failed",
			expectError:       false,
			expectWarning:     true,
		},
		{
			name:        "launch fails with API error",
			launchFails: true,
			expectError: true,
		},
		{
			name:        "invalid JSON response from API",
			invalidJSON: true,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := NewMockProviderHTTPClient(ctrl)
			workflowAction := &WorkflowJobAction{client: mockClient}

			setupWorkflowJobActionMocks(mockClient, tc)
			req := createWorkflowJobActionRequest(t, workflowAction, tc)

			resp := &action.InvokeResponse{
				SendProgress: func(_ action.InvokeProgressEvent) {},
			}

			workflowAction.Invoke(t.Context(), req, resp)

			if tc.expectError && !resp.Diagnostics.HasError() {
				t.Error("expected error but got none")
			}
			if !tc.expectError && resp.Diagnostics.HasError() {
				t.Errorf("unexpected error: %v", resp.Diagnostics.Errors())
			}
			if tc.expectWarning && len(resp.Diagnostics.Warnings()) == 0 {
				t.Error("expected warning but got none")
			}
		})
	}
}

// TestWorkflowJobActionInvokeWithNewFields specifically tests the new prompt-on-launch fields
func TestWorkflowJobActionInvokeWithNewFields(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := NewMockProviderHTTPClient(ctrl)
	workflowAction := &WorkflowJobAction{client: mockClient}

	// Mock GetLaunchWorkflowJob - all fields allowed on launch
	mockClient.EXPECT().getAPIEndpoint().Return("/api/v2").Times(1)
	mockClient.EXPECT().
		doRequest(http.MethodGet, gomock.Any(), nil, nil).
		Return(&http.Response{StatusCode: http.StatusOK}, []byte(`{
			"ask_variables_on_launch": true,
			"ask_limit_on_launch": true,
			"ask_tags_on_launch": true,
			"ask_skip_tags_on_launch": true,
			"ask_labels_on_launch": true
		}`), nil).
		Times(1)

	// Mock LaunchJobTemplate - verify new fields are in the request
	mockClient.EXPECT().getAPIEndpoint().Return("/api/v2").Times(1)
	launchResp := []byte(`{
		"workflow_job_template": 123,
		"url": "/api/v2/workflow_jobs/456/",
		"status": "pending",
		"inventory": 10,
		"limit": "webservers",
		"job_tags": "deploy",
		"skip_tags": "debug"
	}`)
	mockClient.EXPECT().
		doRequest(http.MethodPost, gomock.Any(), nil, gomock.Any()).
		Return(&http.Response{StatusCode: http.StatusCreated}, launchResp, nil).
		Times(1)

	ctx := t.Context()

	// Create schema
	schemaReq := action.SchemaRequest{}
	schemaResp := &action.SchemaResponse{}
	workflowAction.Schema(ctx, schemaReq, schemaResp)

	configType := schemaResp.Schema.Type().TerraformType(ctx)
	configVal := tftypes.NewValue(configType, map[string]tftypes.Value{
		"workflow_job_template_id":            tftypes.NewValue(tftypes.Number, big.NewFloat(123)),
		"inventory_id":                        tftypes.NewValue(tftypes.Number, big.NewFloat(10)),
		"wait_for_completion":                 tftypes.NewValue(tftypes.Bool, false),
		"wait_for_completion_timeout_seconds": tftypes.NewValue(tftypes.Number, big.NewFloat(120)),
		"ignore_job_results":                  tftypes.NewValue(tftypes.Bool, false),
		"extra_vars":                          tftypes.NewValue(tftypes.String, `{"key":"value"}`),
		"limit":                               tftypes.NewValue(tftypes.String, "webservers"),
		"job_tags":                            tftypes.NewValue(tftypes.String, "deploy"),
		"skip_tags":                           tftypes.NewValue(tftypes.String, "debug"),
		"labels": tftypes.NewValue(tftypes.List{ElementType: tftypes.Number}, []tftypes.Value{
			tftypes.NewValue(tftypes.Number, big.NewFloat(5)),
		}),
	})

	req := action.InvokeRequest{
		Config: tfsdk.Config{
			Raw:    configVal,
			Schema: schemaResp.Schema,
		},
	}

	resp := &action.InvokeResponse{
		SendProgress: func(_ action.InvokeProgressEvent) {},
	}

	workflowAction.Invoke(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Errorf("unexpected error: %v", resp.Diagnostics.Errors())
	}
}
