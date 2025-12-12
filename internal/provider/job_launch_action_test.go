package provider

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"

	fwaction "github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"go.uber.org/mock/gomock"
)

func TestJobLaunchActionSchema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schemaRequest := fwaction.SchemaRequest{}
	schemaResponse := fwaction.SchemaResponse{}

	NewJobAction().Schema(ctx, schemaRequest, &schemaResponse)

	if schemaResponse.Diagnostics.HasError() {
		t.Fatalf("Schema method diagnostics: %+v", schemaResponse.Diagnostics)
	}

	diagnostics := schemaResponse.Schema.ValidateImplementation(ctx)

	if diagnostics.HasError() {
		t.Fatalf("Schema validation diagnostics: %+v", diagnostics)
	}
}

func TestJobLaunchActionMetadata(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	metadataRequest := fwaction.MetadataRequest{
		ProviderTypeName: "aap",
	}
	metadataResponse := fwaction.MetadataResponse{}

	NewJobAction().Metadata(ctx, metadataRequest, &metadataResponse)

	expected := "aap_job_launch"
	actual := metadataResponse.TypeName
	if expected != actual {
		t.Errorf("Expected metadata TypeName %q, received %q", expected, actual)
	}
}

func TestJobLaunchActionConfigure(t *testing.T) {
	t.Parallel()

	testTable := []struct {
		name             string
		providerData     interface{}
		expectError      bool
		expectedErrorMsg string
	}{
		{
			name:         "valid provider data",
			providerData: &AAPClient{},
			expectError:  false,
		},
		{
			name:         "nil provider data",
			providerData: nil,
			expectError:  false,
		},
		{
			name:             "invalid provider data type",
			providerData:     "invalid",
			expectError:      true,
			expectedErrorMsg: "Unexpected Resource Configure Type",
		},
	}

	for _, tc := range testTable {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			act := &JobAction{}
			configureRequest := fwaction.ConfigureRequest{
				ProviderData: tc.providerData,
			}
			configureResponse := fwaction.ConfigureResponse{}

			act.Configure(ctx, configureRequest, &configureResponse)

			if tc.expectError {
				if !configureResponse.Diagnostics.HasError() {
					t.Errorf("Expected error but got none")
				}
				if tc.expectedErrorMsg != "" {
					found := false
					for _, diag := range configureResponse.Diagnostics.Errors() {
						if strings.Contains(diag.Summary(), tc.expectedErrorMsg) {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected error message containing %q, got: %v", tc.expectedErrorMsg, configureResponse.Diagnostics.Errors())
					}
				}
			} else if configureResponse.Diagnostics.HasError() {
				t.Errorf("Unexpected error: %v", configureResponse.Diagnostics.Errors())
			}
		})
	}
}

// jobActionConfigOverrides allows overriding specific config values
type jobActionConfigOverrides struct {
	TemplateID               *int64
	InventoryID              *int64
	ExtraVars                *string
	WaitForCompletion        *bool
	WaitForCompletionTimeout *int64
	IgnoreJobResults         *bool
}

// valueOrNil returns a tftypes.Value with the given value if non-nil, otherwise a typed nil
func valueOrNil[T any](tfType tftypes.Type, value *T) tftypes.Value {
	if value != nil {
		return tftypes.NewValue(tfType, *value)
	}
	return tftypes.NewValue(tfType, nil)
}

// createJobActionConfig creates a config map with defaults and optional overrides
func createJobActionConfig(overrides jobActionConfigOverrides) map[string]tftypes.Value {
	return map[string]tftypes.Value{
		"job_template_id":                     valueOrNil(tftypes.Number, overrides.TemplateID),
		"inventory_id":                        valueOrNil(tftypes.Number, overrides.InventoryID),
		"extra_vars":                          valueOrNil(tftypes.String, overrides.ExtraVars),
		"wait_for_completion":                 valueOrNil(tftypes.Bool, overrides.WaitForCompletion),
		"wait_for_completion_timeout_seconds": valueOrNil(tftypes.Number, overrides.WaitForCompletionTimeout),
		"ignore_job_results":                  valueOrNil(tftypes.Bool, overrides.IgnoreJobResults),
	}
}

// mockSuccessfulJobLaunch mocks a successful job launch API call
func mockSuccessfulJobLaunch(mock *MockProviderHTTPClient) {
	mock.EXPECT().getAPIEndpoint().Return("/api/v2")
	mock.EXPECT().doRequest(
		http.MethodPost,
		gomock.Any(),
		gomock.Nil(),
		gomock.Any(),
	).Return(&http.Response{StatusCode: http.StatusCreated}, []byte(`{
		"url": "/api/v2/jobs/789/",
		"status": "pending",
		"type": "job",
		"job_template": 123,
		"extra_vars": "{}",
		"ignored_fields": {}
	}`), nil)
}

// mockFailedJobLaunch mocks a failed job launch API call
func mockFailedJobLaunch(mock *MockProviderHTTPClient, statusCode int, responseBody string, err error) {
	mock.EXPECT().getAPIEndpoint().Return("/api/v2")
	mock.EXPECT().doRequest(
		http.MethodPost,
		gomock.Any(),
		gomock.Nil(),
		gomock.Any(),
	).Return(&http.Response{StatusCode: statusCode}, []byte(responseBody), err)
}

// Helper function to create an InvokeRequest with config data
func createInvokeRequest(t *testing.T, action *JobAction, configValues map[string]tftypes.Value) fwaction.InvokeRequest {
	t.Helper()

	ctx := context.Background()
	schemaReq := fwaction.SchemaRequest{}
	schemaResp := fwaction.SchemaResponse{}
	action.Schema(ctx, schemaReq, &schemaResp)

	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("Failed to get schema: %v", schemaResp.Diagnostics)
	}

	configType := schemaResp.Schema.Type().TerraformType(ctx)
	configVal := tftypes.NewValue(configType, configValues)

	config := tfsdk.Config{
		Raw:    configVal,
		Schema: schemaResp.Schema,
	}

	return fwaction.InvokeRequest{
		Config: config,
	}
}

// createMockInvokeResponse creates an InvokeResponse with a mocked SendProgress function
func createMockInvokeResponse(t *testing.T) *fwaction.InvokeResponse {
	t.Helper()

	progressMessages := []string{}

	return &fwaction.InvokeResponse{
		SendProgress: func(event fwaction.InvokeProgressEvent) {
			progressMessages = append(progressMessages, event.Message)
			t.Logf("Progress: %s", event.Message)
		},
	}
}

// TestJobLaunchActionInvoke tests the full Invoke method
func TestJobLaunchActionInvoke(t *testing.T) {
	t.Parallel()

	templateID := int64(123)
	timeout := int64(5)
	waitTrue := true
	waitFalse := false
	ignoreTrue := true
	ignoreFalse := false

	testTable := []struct {
		name             string
		configOverrides  jobActionConfigOverrides
		setupMock        func(*MockProviderHTTPClient)
		expectError      bool
		expectWarning    bool
		expectedErrorMsg string
	}{
		{
			name: "fire and forget - successful launch",
			configOverrides: jobActionConfigOverrides{
				TemplateID:        &templateID,
				WaitForCompletion: &waitFalse,
			},
			setupMock: func(mock *MockProviderHTTPClient) {
				mockSuccessfulJobLaunch(mock)
			},
			expectError: false,
		},
		{
			name: "launch fails with API error",
			configOverrides: jobActionConfigOverrides{
				TemplateID:        &templateID,
				WaitForCompletion: &waitFalse,
			},
			setupMock: func(mock *MockProviderHTTPClient) {
				mockFailedJobLaunch(mock, http.StatusBadRequest, `{"error": "bad request"}`, errors.New("API error"))
			},
			expectError:      true,
			expectedErrorMsg: "API error",
		},
		{
			name: "invalid JSON response from API",
			configOverrides: jobActionConfigOverrides{
				TemplateID:        &templateID,
				WaitForCompletion: &waitFalse,
			},
			setupMock: func(mock *MockProviderHTTPClient) {
				mockFailedJobLaunch(mock, http.StatusCreated, `invalid json`, nil)
			},
			expectError:      true,
			expectedErrorMsg: "Error parsing JSON response from AAP",
		},
		{
			name: "wait for completion - job succeeds",
			configOverrides: jobActionConfigOverrides{
				TemplateID:               &templateID,
				WaitForCompletion:        &waitTrue,
				WaitForCompletionTimeout: &timeout,
			},
			setupMock: func(mock *MockProviderHTTPClient) {
				mockSuccessfulJobLaunch(mock)
				mock.EXPECT().Get("/api/v2/jobs/789/").Return([]byte(`{"status": "successful"}`), nil)
			},
			expectError: false,
		},
		{
			name: "wait for completion - job fails - no ignore",
			configOverrides: jobActionConfigOverrides{
				TemplateID:               &templateID,
				WaitForCompletion:        &waitTrue,
				WaitForCompletionTimeout: &timeout,
				IgnoreJobResults:         &ignoreFalse,
			},
			setupMock: func(mock *MockProviderHTTPClient) {
				mockSuccessfulJobLaunch(mock)
				mock.EXPECT().Get("/api/v2/jobs/789/").Return([]byte(`{"status": "failed"}`), nil)
			},
			expectError:      true,
			expectedErrorMsg: "AAP job failed",
		},
		{
			name: "wait for completion - job fails - with ignore",
			configOverrides: jobActionConfigOverrides{
				TemplateID:               &templateID,
				WaitForCompletion:        &waitTrue,
				WaitForCompletionTimeout: &timeout,
				IgnoreJobResults:         &ignoreTrue,
			},
			setupMock: func(mock *MockProviderHTTPClient) {
				mockSuccessfulJobLaunch(mock)
				mock.EXPECT().Get("/api/v2/jobs/789/").Return([]byte(`{"status": "failed"}`), nil)
			},
			expectError:   false,
			expectWarning: true,
		},
		{
			name: "wait for completion - job canceled",
			configOverrides: jobActionConfigOverrides{
				TemplateID:               &templateID,
				WaitForCompletion:        &waitTrue,
				WaitForCompletionTimeout: &timeout,
			},
			setupMock: func(mock *MockProviderHTTPClient) {
				mockSuccessfulJobLaunch(mock)
				mock.EXPECT().Get("/api/v2/jobs/789/").Return([]byte(`{"status": "canceled"}`), nil)
			},
			expectError:      true,
			expectedErrorMsg: "AAP job canceled",
		},
		{
			name: "wait for completion - uses default timeout",
			configOverrides: jobActionConfigOverrides{
				TemplateID:        &templateID,
				WaitForCompletion: &waitTrue,
			},
			setupMock: func(mock *MockProviderHTTPClient) {
				mockSuccessfulJobLaunch(mock)
				mock.EXPECT().Get("/api/v2/jobs/789/").Return([]byte(`{"status": "successful"}`), nil)
			},
			expectError: false,
		},
	}

	for _, tc := range testTable {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := NewMockProviderHTTPClient(ctrl)
			tc.setupMock(mockClient)

			action := &JobAction{
				client: mockClient,
			}

			configValues := createJobActionConfig(tc.configOverrides)

			req := createInvokeRequest(t, action, configValues)
			resp := createMockInvokeResponse(t)

			ctx := context.Background()
			action.Invoke(ctx, req, resp)

			hasError := resp.Diagnostics.HasError()
			hasWarning := resp.Diagnostics.WarningsCount() > 0

			if tc.expectError && !hasError {
				t.Errorf("Expected error but got none")
			}

			if !tc.expectError && hasError {
				t.Errorf("Unexpected error: %v", resp.Diagnostics.Errors())
			}

			if tc.expectWarning && !hasWarning {
				t.Errorf("Expected warning but got none")
			}

			if tc.expectedErrorMsg != "" && hasError {
				found := false
				for _, diag := range resp.Diagnostics.Errors() {
					if strings.Contains(diag.Summary(), tc.expectedErrorMsg) || strings.Contains(diag.Detail(), tc.expectedErrorMsg) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected error message containing %q, got: %v", tc.expectedErrorMsg, resp.Diagnostics.Errors())
				}
			}
		})
	}
}

func TestAccAAPJobAction_basic(t *testing.T) {
	jobTemplateID := os.Getenv("AAP_TEST_JOB_TEMPLATE_ID")
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
		PreCheck:                 func() { testAccJobResourcePreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccBasicJobAction(inventoryName, jobTemplateID),
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
		if strings.Contains(logLine, "job launched") {
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

func TestAccAAPJobAction_fail(t *testing.T) {
	jobTemplateID := os.Getenv("AAP_TEST_JOB_TEMPLATE_FAIL_ID")
	randNum, _ := rand.Int(rand.Reader, big.NewInt(50000000))
	inventoryName := fmt.Sprintf("%s-%d", "tf-acc", randNum.Int64())

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccJobResourcePreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccBasicJobAction(inventoryName, jobTemplateID),
				ExpectError: regexp.MustCompile(".*AAP job failed.*"),
			},
		},
	})
}

func TestAccAAPJobAction_failIgnore(t *testing.T) {
	jobTemplateID := os.Getenv("AAP_TEST_JOB_TEMPLATE_FAIL_ID")
	randNum, _ := rand.Int(rand.Reader, big.NewInt(50000000))
	inventoryName := fmt.Sprintf("%s-%d", "tf-acc", randNum.Int64())

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccJobResourcePreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccBasicJobActionIgnoreFail(inventoryName, jobTemplateID),
			},
		},
	})
}

func testAccBasicJobAction(inventoryName, jobTemplateID string) string {
	return fmt.Sprintf(`
resource "aap_inventory" "test" {
	name = "%s"
	lifecycle {
		action_trigger {
			events = [after_create]
			actions = [action.aap_job_launch.test]
		}
	}
}

action "aap_job_launch" "test" {
	config {
		job_template_id 	= %s
		wait_for_completion = true
	}
}
`, inventoryName, jobTemplateID)
}

func testAccBasicJobActionIgnoreFail(inventoryName, jobTemplateID string) string {
	return fmt.Sprintf(`
resource "aap_inventory" "test" {
	name = "%s"
	lifecycle {
		action_trigger {
			events = [after_create]
			actions = [action.aap_job_launch.test]
		}
	}
}

action "aap_job_launch" "test" {
	config {
		job_template_id 	= %s
		wait_for_completion = true
		ignore_job_results  = true
	}
}
`, inventoryName, jobTemplateID)
}
