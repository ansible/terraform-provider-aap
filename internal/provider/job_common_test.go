package provider

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/ansible/terraform-provider-aap/internal/provider/customtypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"go.uber.org/mock/gomock"
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
