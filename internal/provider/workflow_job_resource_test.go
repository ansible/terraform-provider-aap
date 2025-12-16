package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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
	"go.uber.org/mock/gomock"
)

const (
	baseResourceNameWorkflowJob = "aap_workflow_job"
)

func TestWorkflowJobResourceSchema(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
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
				WorkflowJobModel: WorkflowJobModel{
					ExtraVars:   customtypes.NewAAPCustomStringUnknown(),
					InventoryID: basetypes.NewInt64Unknown(),
					TemplateID:  types.Int64Value(1),
				},
			},
			expected: []byte(`{}`),
		},
		{
			name: "null values",
			input: WorkflowJobResourceModel{
				WorkflowJobModel: WorkflowJobModel{
					ExtraVars:   customtypes.NewAAPCustomStringNull(),
					InventoryID: basetypes.NewInt64Null(),
					TemplateID:  types.Int64Value(1),
				},
			},
			expected: []byte(`{}`),
		},
		{
			name: "extra vars only",
			input: WorkflowJobResourceModel{
				WorkflowJobModel: WorkflowJobModel{
					ExtraVars:   customtypes.NewAAPCustomStringValue("{\"test_name\":\"extra_vars\", \"provider\":\"aap\"}"),
					InventoryID: basetypes.NewInt64Null(),
				},
			},
			expected: []byte(`{"extra_vars":"{\"test_name\":\"extra_vars\", \"provider\":\"aap\"}"}`),
		},
		{
			name: "inventory vars only",
			input: WorkflowJobResourceModel{
				WorkflowJobModel: WorkflowJobModel{
					ExtraVars:   customtypes.NewAAPCustomStringNull(),
					InventoryID: basetypes.NewInt64Value(201),
				},
			},
			expected: []byte(`{"inventory": 201}`),
		},
		{
			name: "combined",
			input: WorkflowJobResourceModel{
				WorkflowJobModel: WorkflowJobModel{
					ExtraVars:   customtypes.NewAAPCustomStringValue("{\"test_name\":\"extra_vars\", \"provider\":\"aap\"}"),
					InventoryID: basetypes.NewInt64Value(3),
				},
			},
			expected: []byte(`{"inventory":3,"extra_vars":"{\"test_name\":\"extra_vars\", \"provider\":\"aap\"}"}`),
		},
		{
			name: "manual_triggers",
			input: WorkflowJobResourceModel{
				WorkflowJobModel: WorkflowJobModel{
					InventoryID: basetypes.NewInt64Value(3),
				},
				Triggers: types.MapNull(types.StringType),
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

func TestWorkflowJobResourceParseHTTPResponse(t *testing.T) {
	templateID := basetypes.NewInt64Value(1)
	inventoryID := basetypes.NewInt64Value(2)
	extraVars := customtypes.NewAAPCustomStringNull()
	limit := customtypes.NewAAPCustomStringValue("")
	jobTags := customtypes.NewAAPCustomStringValue("")
	skipTags := customtypes.NewAAPCustomStringValue("")
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
				WorkflowJobModel: WorkflowJobModel{
					TemplateID:  templateID,
					InventoryID: inventoryID,
					ExtraVars:   extraVars,
					Limit:       limit,
					JobTags:     jobTags,
					SkipTags:    skipTags,
				},
				Type:          types.StringValue("run"),
				URL:           types.StringValue("/api/v2/workflow_jobs/14/"),
				Status:        types.StringValue("pending"),
				IgnoredFields: types.ListNull(types.StringType),
			},
			errors: diag.Diagnostics{},
		},
		{
			name: "ignored fields",
			input: []byte(`{"inventory":2,"workflow_job_template":1,"job_type": "run", "url": "/api/v2/workflow_jobs/14/", "status":
			"pending", "ignored_fields": {"extra_vars": "{\"bucket_state\":\"absent\"}"}}`),
			expected: WorkflowJobResourceModel{
				WorkflowJobModel: WorkflowJobModel{
					TemplateID:  templateID,
					InventoryID: inventoryID,
					ExtraVars:   extraVars,
					Limit:       limit,
					JobTags:     jobTags,
					SkipTags:    skipTags,
				},
				Type:          types.StringValue("run"),
				URL:           types.StringValue("/api/v2/workflow_jobs/14/"),
				Status:        types.StringValue("pending"),
				IgnoredFields: basetypes.NewListValueMust(types.StringType, []attr.Value{types.StringValue("extra_vars")}),
			},
			errors: diag.Diagnostics{},
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			resource := WorkflowJobResourceModel{}
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

func TestWorkflowJobResourceMetadata(t *testing.T) {
	resource := NewWorkflowJobResource()
	req := fwresource.MetadataRequest{
		ProviderTypeName: "aap",
	}
	resp := &fwresource.MetadataResponse{}

	resource.Metadata(t.Context(), req, resp)

	expectedTypeName := baseResourceNameWorkflowJob
	if resp.TypeName != expectedTypeName {
		t.Errorf("expected TypeName to be %q, got %q", expectedTypeName, resp.TypeName)
	}
}

func TestWorkflowJobResourceConfigure(t *testing.T) {
	t.Parallel()

	mockClient := &AAPClient{}

	testCases := []struct {
		name          string
		providerData  interface{}
		expectClient  interface{}
		expectDiagErr bool
	}{
		{
			name:          "ProviderData is nil",
			providerData:  nil,
			expectClient:  nil,
			expectDiagErr: false,
		},
		{
			name:          "ProviderData is wrong type",
			providerData:  "not-a-client",
			expectClient:  nil,
			expectDiagErr: true,
		},
		{
			name:          "ProviderData is correct type",
			providerData:  mockClient,
			expectClient:  mockClient,
			expectDiagErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resource := &WorkflowJobResource{}
			req := fwresource.ConfigureRequest{
				ProviderData: tc.providerData,
			}
			resp := &fwresource.ConfigureResponse{}
			resource.Configure(t.Context(), req, resp)

			if resource.client != tc.expectClient {
				t.Errorf("expected client to be %v, got %v", tc.expectClient, resource.client)
			}
			if resp.Diagnostics.HasError() != tc.expectDiagErr {
				t.Errorf("expected diagnostics error: %v, got: %v", tc.expectDiagErr, resp.Diagnostics.HasError())
			}
		})
	}
}

func TestWorkflowJobModelCreateRequestBody(t *testing.T) {
	testTable := []struct {
		name     string
		input    WorkflowJobModel
		expected []byte
	}{
		{
			name: "unknown values",
			input: WorkflowJobModel{
				ExtraVars:   customtypes.NewAAPCustomStringUnknown(),
				InventoryID: basetypes.NewInt64Unknown(),
				TemplateID:  types.Int64Value(1),
			},
			expected: []byte(`{}`),
		},
		{
			name: "null values",
			input: WorkflowJobModel{
				ExtraVars:   customtypes.NewAAPCustomStringNull(),
				InventoryID: basetypes.NewInt64Null(),
				TemplateID:  types.Int64Value(1),
			},
			expected: []byte(`{}`),
		},
		{
			name: "extra vars only",
			input: WorkflowJobModel{
				ExtraVars:   customtypes.NewAAPCustomStringValue("{\"test_name\":\"extra_vars\", \"provider\":\"aap\"}"),
				InventoryID: basetypes.NewInt64Null(),
			},
			expected: []byte(`{"extra_vars":"{\"test_name\":\"extra_vars\", \"provider\":\"aap\"}"}`),
		},
		{
			name: "inventory only",
			input: WorkflowJobModel{
				ExtraVars:   customtypes.NewAAPCustomStringNull(),
				InventoryID: basetypes.NewInt64Value(201),
			},
			expected: []byte(`{"inventory": 201}`),
		},
		{
			name: "combined extra vars and inventory",
			input: WorkflowJobModel{
				ExtraVars:   customtypes.NewAAPCustomStringValue("{\"test_name\":\"extra_vars\", \"provider\":\"aap\"}"),
				InventoryID: basetypes.NewInt64Value(3),
			},
			expected: []byte(`{"inventory":3,"extra_vars":"{\"test_name\":\"extra_vars\", \"provider\":\"aap\"}"}`),
		},
		{
			name: "wait_for_completed parameters not included in request",
			input: WorkflowJobModel{
				InventoryID:              basetypes.NewInt64Value(3),
				TemplateID:               types.Int64Value(1),
				WaitForCompletion:        basetypes.NewBoolValue(true),
				WaitForCompletionTimeout: basetypes.NewInt64Value(60),
			},
			expected: []byte(`{"inventory":3}`),
		},
		{
			name: "labels serialization",
			input: WorkflowJobModel{
				TemplateID: types.Int64Value(1),
				Labels:     basetypes.NewListValueMust(types.Int64Type, []attr.Value{types.Int64Value(10), types.Int64Value(20)}),
			},
			expected: []byte(`{"labels":[10,20]}`),
		},
		{
			name: "limit only",
			input: WorkflowJobModel{
				TemplateID: types.Int64Value(1),
				Limit:      customtypes.NewAAPCustomStringValue("webservers"),
			},
			expected: []byte(`{"limit":"webservers"}`),
		},
		{
			name: "job_tags only",
			input: WorkflowJobModel{
				TemplateID: types.Int64Value(1),
				JobTags:    customtypes.NewAAPCustomStringValue("deploy,install"),
			},
			expected: []byte(`{"job_tags":"deploy,install"}`),
		},
		{
			name: "skip_tags only",
			input: WorkflowJobModel{
				TemplateID: types.Int64Value(1),
				SkipTags:   customtypes.NewAAPCustomStringValue("debug,test"),
			},
			expected: []byte(`{"skip_tags":"debug,test"}`),
		},
		{
			name: "all prompt-on-launch fields",
			input: WorkflowJobModel{
				TemplateID:  types.Int64Value(1),
				InventoryID: basetypes.NewInt64Value(100),
				Labels:      basetypes.NewListValueMust(types.Int64Type, []attr.Value{types.Int64Value(5)}),
				ExtraVars:   customtypes.NewAAPCustomStringValue(`{"key":"value"}`),
				Limit:       customtypes.NewAAPCustomStringValue("webservers"),
				JobTags:     customtypes.NewAAPCustomStringValue("deploy"),
				SkipTags:    customtypes.NewAAPCustomStringValue("debug"),
			},
			expected: []byte(`{"inventory":100,"extra_vars":"{\"key\":\"value\"}","limit":"webservers","job_tags":"deploy","skip_tags":"debug","labels":[5]}`),
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

func TestWorkflowJobModelGetLaunchWorkflowJob(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		templateID     int64
		mockResponse   []byte
		mockStatusCode int
		expectError    bool
		validateResult func(t *testing.T, config WorkflowJobLaunchAPIModel)
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
				"ask_labels_on_launch": true
			}`),
			expectError: false,
			validateResult: func(t *testing.T, config WorkflowJobLaunchAPIModel) {
				if !config.AskVariablesOnLaunch {
					t.Error("expected AskVariablesOnLaunch to be true")
				}
				if !config.AskLimitOnLaunch {
					t.Error("expected AskLimitOnLaunch to be true")
				}
				if config.AskTagsOnLaunch {
					t.Error("expected AskTagsOnLaunch to be false")
				}
				if !config.AskLabelsOnLaunch {
					t.Error("expected AskLabelsOnLaunch to be true")
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
				resp = createMockResponse(tc.mockStatusCode, http.MethodGet, "/api/v2/workflow_job_templates/"+types.Int64Value(tc.templateID).String()+"/launch")
			} else {
				resp = &http.Response{StatusCode: tc.mockStatusCode}
			}

			mockClient.EXPECT().
				doRequest(http.MethodGet, gomock.Any(), nil, nil).
				Return(resp, tc.mockResponse, nil)

			model := &WorkflowJobModel{TemplateID: types.Int64Value(tc.templateID)}
			config, diags := model.GetLaunchWorkflowJob(mockClient)

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

func TestWorkflowJobModelCanWorkflowJobBeLaunched(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		launchConfig   WorkflowJobLaunchAPIModel
		model          WorkflowJobModel
		expectError    bool
		expectWarnings bool
	}{
		// Base case
		{
			name:         "all fields optional - no errors",
			launchConfig: WorkflowJobLaunchAPIModel{},
			model:        WorkflowJobModel{TemplateID: types.Int64Value(1)},
			expectError:  false,
		},
		// extra_vars
		{
			name:         "extra_vars required but not provided",
			launchConfig: WorkflowJobLaunchAPIModel{AskVariablesOnLaunch: true},
			model:        WorkflowJobModel{TemplateID: types.Int64Value(1), ExtraVars: customtypes.NewAAPCustomStringNull()},
			expectError:  true,
		},
		{
			name:           "extra_vars provided but not expected - warning",
			launchConfig:   WorkflowJobLaunchAPIModel{AskVariablesOnLaunch: false},
			model:          WorkflowJobModel{TemplateID: types.Int64Value(1), ExtraVars: customtypes.NewAAPCustomStringValue(`{"key": "value"}`)},
			expectWarnings: true,
		},
		// inventory_id
		{
			name:         "inventory_id required but not provided",
			launchConfig: WorkflowJobLaunchAPIModel{AskInventoryOnLaunch: true},
			model:        WorkflowJobModel{TemplateID: types.Int64Value(1), InventoryID: types.Int64Null()},
			expectError:  true,
		},
		{
			name:           "inventory_id provided but not expected - warning",
			launchConfig:   WorkflowJobLaunchAPIModel{AskInventoryOnLaunch: false},
			model:          WorkflowJobModel{TemplateID: types.Int64Value(1), InventoryID: types.Int64Value(10)},
			expectWarnings: true,
		},
		// limit
		{
			name:         "limit required but not provided",
			launchConfig: WorkflowJobLaunchAPIModel{AskLimitOnLaunch: true},
			model:        WorkflowJobModel{TemplateID: types.Int64Value(1), Limit: customtypes.NewAAPCustomStringNull()},
			expectError:  true,
		},
		{
			name:           "limit provided but not expected - warning",
			launchConfig:   WorkflowJobLaunchAPIModel{AskLimitOnLaunch: false},
			model:          WorkflowJobModel{TemplateID: types.Int64Value(1), Limit: customtypes.NewAAPCustomStringValue("all")},
			expectWarnings: true,
		},
		// job_tags
		{
			name:         "job_tags required but not provided",
			launchConfig: WorkflowJobLaunchAPIModel{AskTagsOnLaunch: true},
			model:        WorkflowJobModel{TemplateID: types.Int64Value(1), JobTags: customtypes.NewAAPCustomStringNull()},
			expectError:  true,
		},
		{
			name:           "job_tags provided but not expected - warning",
			launchConfig:   WorkflowJobLaunchAPIModel{AskTagsOnLaunch: false},
			model:          WorkflowJobModel{TemplateID: types.Int64Value(1), JobTags: customtypes.NewAAPCustomStringValue("deploy")},
			expectWarnings: true,
		},
		// skip_tags
		{
			name:         "skip_tags required but not provided",
			launchConfig: WorkflowJobLaunchAPIModel{AskSkipTagsOnLaunch: true},
			model:        WorkflowJobModel{TemplateID: types.Int64Value(1), SkipTags: customtypes.NewAAPCustomStringNull()},
			expectError:  true,
		},
		{
			name:           "skip_tags provided but not expected - warning",
			launchConfig:   WorkflowJobLaunchAPIModel{AskSkipTagsOnLaunch: false},
			model:          WorkflowJobModel{TemplateID: types.Int64Value(1), SkipTags: customtypes.NewAAPCustomStringValue("debug")},
			expectWarnings: true,
		},
		// labels
		{
			name:         "labels required but not provided",
			launchConfig: WorkflowJobLaunchAPIModel{AskLabelsOnLaunch: true},
			model:        WorkflowJobModel{TemplateID: types.Int64Value(1), Labels: types.ListNull(types.Int64Type)},
			expectError:  true,
		},
		{
			name:           "labels provided but not expected - warning",
			launchConfig:   WorkflowJobLaunchAPIModel{AskLabelsOnLaunch: false},
			model:          WorkflowJobModel{TemplateID: types.Int64Value(1), Labels: basetypes.NewListValueMust(types.Int64Type, []attr.Value{types.Int64Value(1)})},
			expectWarnings: true,
		},
		// Combined success case
		{
			name: "all required fields provided - no errors",
			launchConfig: WorkflowJobLaunchAPIModel{
				AskVariablesOnLaunch: true,
				AskLimitOnLaunch:     true,
				AskInventoryOnLaunch: true,
				AskLabelsOnLaunch:    true,
			},
			model: WorkflowJobModel{
				TemplateID:  types.Int64Value(1),
				ExtraVars:   customtypes.NewAAPCustomStringValue(`{"key": "value"}`),
				Limit:       customtypes.NewAAPCustomStringValue("all"),
				InventoryID: types.Int64Value(10),
				Labels:      basetypes.NewListValueMust(types.Int64Type, []attr.Value{types.Int64Value(5)}),
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

			diags := tc.model.CanWorkflowJobBeLaunched(mockClient)

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

func TestWorkflowJobModelLaunchWorkflowJob(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		model          WorkflowJobModel
		launchConfig   WorkflowJobLaunchAPIModel
		postStatusCode int
		postResponse   []byte
		expectError    bool
		skipPostMock   bool // If CanWorkflowJobBeLaunched fails, POST is not called
	}{
		{
			name:         "successful workflow job launch",
			model:        WorkflowJobModel{TemplateID: types.Int64Value(123)},
			launchConfig: WorkflowJobLaunchAPIModel{
				// All fields optional
			},
			postStatusCode: http.StatusCreated,
			postResponse: []byte(`{
				"id": 456,
				"url": "/api/v2/workflow_jobs/456/",
				"status": "pending"
			}`),
			expectError: false,
		},
		{
			name: "launch fails when CanWorkflowJobBeLaunched fails",
			model: WorkflowJobModel{
				TemplateID: types.Int64Value(123),
				ExtraVars:  customtypes.NewAAPCustomStringNull(),
			},
			launchConfig: WorkflowJobLaunchAPIModel{
				AskVariablesOnLaunch: true, // extra_vars required but not provided
			},
			expectError:  true,
			skipPostMock: true,
		},
		{
			name:           "launch fails when POST fails",
			model:          WorkflowJobModel{TemplateID: types.Int64Value(123)},
			launchConfig:   WorkflowJobLaunchAPIModel{},
			postStatusCode: http.StatusInternalServerError,
			postResponse:   []byte(`{"error": "server error"}`),
			expectError:    true,
		},
		{
			name: "launch with all parameters",
			model: WorkflowJobModel{
				TemplateID:  types.Int64Value(123),
				InventoryID: types.Int64Value(10),
				ExtraVars:   customtypes.NewAAPCustomStringValue(`{"env": "prod"}`),
				Limit:       customtypes.NewAAPCustomStringValue("webservers"),
				Labels:      basetypes.NewListValueMust(types.Int64Type, []attr.Value{types.Int64Value(5)}),
			},
			launchConfig: WorkflowJobLaunchAPIModel{
				AskVariablesOnLaunch: true,
				AskLimitOnLaunch:     true,
				AskInventoryOnLaunch: true,
				AskLabelsOnLaunch:    true,
			},
			postStatusCode: http.StatusCreated,
			postResponse: []byte(`{
				"id": 789,
				"url": "/api/v2/workflow_jobs/789/",
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

			// GetLaunchWorkflowJob mock
			expectedAPICalls := 1
			if !tc.skipPostMock {
				expectedAPICalls = 2
			}
			mockClient.EXPECT().getAPIEndpoint().Return("/api/v2").Times(expectedAPICalls)

			configJSON, _ := json.Marshal(tc.launchConfig)
			mockClient.EXPECT().
				doRequest(http.MethodGet, gomock.Any(), nil, nil).
				Return(&http.Response{StatusCode: http.StatusOK}, configJSON, nil)

			// POST mock (only if CanWorkflowJobBeLaunched passes)
			if !tc.skipPostMock {
				var postResp *http.Response
				if tc.postStatusCode != http.StatusCreated {
					postResp = createMockResponse(tc.postStatusCode, http.MethodPost, "/api/v2/workflow_job_templates/123/launch")
				} else {
					postResp = &http.Response{StatusCode: tc.postStatusCode}
				}
				mockClient.EXPECT().
					doRequest(http.MethodPost, gomock.Any(), nil, gomock.Any()).
					Return(postResp, tc.postResponse, nil)
			}

			body, diags := tc.model.LaunchWorkflowJob(mockClient)

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

// Acceptance tests
func getWorkflowJobResourceFromStateFile(s *terraform.State) (map[string]interface{}, error) {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != baseResourceNameWorkflowJob {
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
			if rs.Type != baseResourceNameWorkflowJob {
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
	ctx := t.Context()

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

func TestAccAAPWorkflowJob_waitForCompletionWithFailure(t *testing.T) {
	jobTemplateID := os.Getenv("AAP_TEST_WORKFLOW_JOB_TEMPLATE_FAIL_ID")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccWorkflowJobResourcePreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccBasicWorkflowJob(jobTemplateID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestMatchResourceAttr("aap_workflow_job.test", "status", regexp.MustCompile("^failed$")),
					resource.TestMatchResourceAttr("aap_workflow_job.test", "url", regexp.MustCompile("^/api(/controller)?/v2/workflow_jobs/[0-9]*/$")),
					testAccCheckWorkflowJobExists,
				),
			},
		},
	})
}

func testAccBasicWorkflowJob(jobTemplateID string) string {
	return fmt.Sprintf(`
resource %q "test" {
	workflow_job_template_id  = %s
	wait_for_completion       = true
}
`, baseResourceNameWorkflowJob, jobTemplateID)
}

func testAccWorkflowJobWithNoInventoryID(workflowJobTemplateID string) string {
	return fmt.Sprintf(`
resource "%s" "wf_job" {
	workflow_job_template_id = %s
	extra_vars = jsonencode({
    "foo": "bar"
	})
}
	`, baseResourceNameWorkflowJob, workflowJobTemplateID)
}

func testAccUpdateWorkflowJobWithInventoryID(inventoryName, jobTemplateID string) string {
	return fmt.Sprintf(`
resource "aap_inventory" "test" {
  name = "%s"
}

resource "%s" "test" {
	workflow_job_template_id   = %s
	inventory_id = aap_inventory.test.id
}
`, inventoryName, baseResourceNameWorkflowJob, jobTemplateID)
}

func testAccUpdateWorkflowJobWithTrigger(jobTemplateID string) string {
	return fmt.Sprintf(`
resource "%s" "test" {
	workflow_job_template_id   = %s
	triggers = {
		"key1" = "value1"
		"key2" = "value2"
	}
}
`, baseResourceNameWorkflowJob, jobTemplateID)
}

// TestAccAAPWorkflowJob_AllFieldsOnPrompt tests that a workflow job resource with all fields on prompt
// can be launched successfully when all required fields are provided.
func TestAccAAPWorkflowJob_AllFieldsOnPrompt(t *testing.T) {
	workflowJobTemplateID := os.Getenv("AAP_TEST_WORKFLOW_JOB_TEMPLATE_ALL_FIELDS_PROMPT_ID")
	if workflowJobTemplateID == "" {
		t.Skip("AAP_TEST_WORKFLOW_JOB_TEMPLATE_ALL_FIELDS_PROMPT_ID environment variable not set")
	}
	labelID := os.Getenv("AAP_TEST_LABEL_ID")
	if labelID == "" {
		t.Skip("AAP_TEST_LABEL_ID environment variable not set")
	}
	inventoryName := acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)
	ctx := t.Context()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccWorkflowJobResourcePreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccWorkflowJobAllFieldsOnPrompt(inventoryName, workflowJobTemplateID, labelID),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckWorkflowJobExists,
					testAccCheckWorkflowJobPause(ctx, "aap_workflow_job.test"),
				),
			},
		},
	})
}

// TestAccAAPWorkflowJob_AllFieldsOnPrompt_MissingRequired tests that a workflow job resource with all
// fields on prompt fails when required fields are not provided.
func TestAccAAPWorkflowJob_AllFieldsOnPrompt_MissingRequired(t *testing.T) {
	workflowJobTemplateID := os.Getenv("AAP_TEST_WORKFLOW_JOB_TEMPLATE_ALL_FIELDS_PROMPT_ID")
	if workflowJobTemplateID == "" {
		t.Skip("AAP_TEST_WORKFLOW_JOB_TEMPLATE_ALL_FIELDS_PROMPT_ID environment variable not set")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccWorkflowJobResourcePreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccWorkflowJobAllFieldsOnPromptMissingRequired(workflowJobTemplateID),
				ExpectError: regexp.MustCompile(".*Missing required field.*"),
			},
		},
	})
}

func testAccWorkflowJobAllFieldsOnPrompt(inventoryName, workflowJobTemplateID, labelID string) string {
	return fmt.Sprintf(`
resource "aap_inventory" "test" {
	name = "%s"
}

resource "aap_workflow_job" "test" {
	workflow_job_template_id = %s
	inventory_id             = aap_inventory.test.id
	labels                   = [%s]
	extra_vars               = "{\"test_var\": \"test_value\"}"
	limit                    = "localhost"
	job_tags                 = "test"
	skip_tags                = "skip"
	wait_for_completion      = true
}
`, inventoryName, workflowJobTemplateID, labelID)
}

func testAccWorkflowJobAllFieldsOnPromptMissingRequired(workflowJobTemplateID string) string {
	return fmt.Sprintf(`
resource "aap_workflow_job" "test" {
	workflow_job_template_id = %s
}
`, workflowJobTemplateID)
}

func TestAccAAPWorkflowJobDisappears(t *testing.T) {
	var workflowJobUrl string

	jobTemplateID := os.Getenv("AAP_TEST_WORKFLOW_JOB_TEMPLATE_ID")
	ctx := t.Context()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccWorkflowJobResourcePreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Apply a basic terraform plan that creates an AAP workflow job and records it to state with a URL.
			{
				Config: testAccBasicWorkflowJob(jobTemplateID),
				Check: resource.ComposeAggregateTestCheckFunc(
					checkBasicWorkflowJobAttributes(t, resourceNameWorkflowJob, reJobStatus),
					testAccCheckWorkflowJobUpdate(&workflowJobUrl, false),
				),
			},
			// Wait for the workflow job to finish.
			{
				Config: testAccBasicWorkflowJob(jobTemplateID),
				Check: resource.ComposeAggregateTestCheckFunc(
					checkBasicWorkflowJobAttributes(t, resourceNameWorkflowJob, reJobStatus),
					// Wait for the workflow job to finish so the inventory can be deleted
					testAccCheckWorkflowJobPause(ctx, resourceNameWorkflowJob),
				),
			},
			// Confirm the workflow job is finished (fewer options in status), then delete directly via API, outside of terraform.
			{
				Config: testAccBasicWorkflowJob(jobTemplateID),
				Check: resource.ComposeAggregateTestCheckFunc(
					checkBasicWorkflowJobAttributes(t, resourceNameWorkflowJob, reJobStatusFinal),
					testAccDeleteJob(&workflowJobUrl),
				),
				ExpectNonEmptyPlan: true,
			},
			// Apply the plan again and confirm the workflow job is re-created with a different URL.
			{
				Config: testAccBasicWorkflowJob(jobTemplateID),
				Check: resource.ComposeAggregateTestCheckFunc(
					checkBasicWorkflowJobAttributes(t, resourceNameWorkflowJob, reJobStatus),
					testAccCheckWorkflowJobUpdate(&workflowJobUrl, true),
				),
			},
		},
	})
}
