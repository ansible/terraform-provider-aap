package provider

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/ansible/terraform-provider-aap/internal/provider/customtypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"go.uber.org/mock/gomock"
)

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
