package provider

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-log/tflogtest"
	"go.uber.org/mock/gomock"
)

// TestBaseEDADataSourceMetadata tests that the Metadata function returns the correct metadata for the
// datasource. This means a string of the form providerName_datasourceName is returned.
func TestBaseEdaDataSourceMetadata(t *testing.T) {
	t.Parallel()

	testDataSource := NewBaseEdaDataSource(nil, StringDescriptions{
		ApiEntitySlug:         "datasourceApiSlug",
		DescriptiveEntityName: "datasourceDescriptiveName",
		MetadataEntitySlug:    "datasourceMetadataSlug",
	})

	ctx := t.Context()
	metadataRequest := datasource.MetadataRequest{
		ProviderTypeName: "provider",
	}
	metadataResponse := &datasource.MetadataResponse{}

	testDataSource.Metadata(ctx, metadataRequest, metadataResponse)

	expected := "provider_datasourceMetadataSlug"
	if metadataResponse.TypeName != expected {
		t.Errorf("Incorrect metadata response. Expected %s. Got %s", expected, metadataResponse.TypeName)
	}
}

// TestBaseEdaDataSourceSchema tests the Schema function results in a proper schema definition.
func TestBaseEdaDataSourceSchema(t *testing.T) {
	t.Parallel()

	testDataSource := NewBaseEdaDataSource(nil, StringDescriptions{
		ApiEntitySlug:         "datasourceApiSlug",
		DescriptiveEntityName: "datasourceDescriptiveName",
		MetadataEntitySlug:    "datasourceMetadataSlug",
	})

	ctx := t.Context()
	schemaRequest := datasource.SchemaRequest{}
	schemaResponse := datasource.SchemaResponse{}

	testDataSource.Schema(ctx, schemaRequest, &schemaResponse)

	if schemaResponse.Diagnostics.HasError() {
		t.Fatalf("Schema method diagnostics contained error: %+v", schemaResponse.Diagnostics)
	}

	diags := schemaResponse.Schema.ValidateImplementation(ctx)
	if diags.HasError() {
		t.Fatalf("Schema validation diagnostics: %+v", diags)
	}
}

// TestBaseEdaDataSourceConfigure tests the Configure takes appropriate action based on the passed context and
// provider data.
func TestBaseEdaDataSourceConfigure(t *testing.T) {
	t.Parallel()

	cancelledContext, cancelFunc := context.WithCancel(t.Context())
	cancelFunc()

	testCases := []struct {
		name                string
		ctx                 context.Context
		expectErrorLog      bool
		expectedLogMessage  string
		expectDiagnosticErr bool
		expectedDiagSummary string
		expectedDiagDetail  string
		request             datasource.ConfigureRequest
		response            *datasource.ConfigureResponse
		expectClientSet     bool
	}{
		{
			name:                "Success scenario",
			ctx:                 t.Context(),
			expectErrorLog:      false,
			expectDiagnosticErr: false,
			request: datasource.ConfigureRequest{
				ProviderData: &AAPClient{}, // Valid client
			},
			response:        &datasource.ConfigureResponse{},
			expectClientSet: true,
		},
		{
			name:                "Response object is nil",
			ctx:                 t.Context(),
			expectErrorLog:      true,
			expectedLogMessage:  "Response not defined, we cannot continue with the execution",
			expectDiagnosticErr: false,
			request:             datasource.ConfigureRequest{},
			response:            nil,
			expectClientSet:     false,
		},
		{
			name:                "Context not active",
			ctx:                 cancelledContext,
			expectErrorLog:      false,
			expectDiagnosticErr: true,
			expectedDiagSummary: "Aborting Configure operation",
			expectedDiagDetail:  "Context is not active, we cannot continue with the execution",
			request:             datasource.ConfigureRequest{},
			response:            &datasource.ConfigureResponse{},
			expectClientSet:     false,
		},
		{
			name:                "ProviderData is nil",
			ctx:                 t.Context(),
			expectErrorLog:      false,
			expectDiagnosticErr: false,
			request:             datasource.ConfigureRequest{ProviderData: nil},
			response:            &datasource.ConfigureResponse{},
			expectClientSet:     false,
		},
		{
			name:                "Wrong ProviderData type",
			ctx:                 t.Context(),
			expectErrorLog:      false,
			expectDiagnosticErr: true,
			expectedDiagSummary: "Unexpected Data Source Configure Type",
			expectedDiagDetail:  "Expected *AAPClient, got: string. Please report this issue to the provider developers.",
			request:             datasource.ConfigureRequest{ProviderData: "wrong"},
			response:            &datasource.ConfigureResponse{},
			expectClientSet:     false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var ctx context.Context
			var logs bytes.Buffer
			if tc.expectErrorLog {
				ctx = tflogtest.RootLogger(tc.ctx, &logs)
			} else {
				ctx = tc.ctx
			}

			testDataSource := NewBaseEdaDataSource(nil, StringDescriptions{
				ApiEntitySlug:         "datasource",
				DescriptiveEntityName: "datasource",
				MetadataEntitySlug:    "datasource",
			})

			testDataSource.Configure(ctx, tc.request, tc.response)

			checkTerraformLogs(t, &logs, tc.expectErrorLog, tc.expectedLogMessage)
			checkTerraformDiagnostics(t, tc.expectDiagnosticErr, getDiags(tc.response), tc.expectedDiagSummary, tc.expectedDiagDetail)

			// Check if client was set (indicates function didn't exit early)
			if tc.expectClientSet && testDataSource.client == nil {
				t.Error("Expected client to be set but it wasn't")
			}
			if !tc.expectClientSet && testDataSource.client != nil {
				t.Error("Expected client to not be set but it was")
			}
		})
	}
}

// TestBaseEdaDataSourceRead tests that the Read function correctly fetches EDA resources by name from the API
// and properly populates the Terraform state with the returned data.
func TestBaseEdaDataSourceRead(t *testing.T) {
	t.Parallel()

	const eventStreamName string = "myeventstream"

	// Create a data source and its schema for use in creating requests and responses
	// in various test cases below.
	baseEdaDataSource := NewBaseEdaDataSource(nil, StringDescriptions{})
	baseEdaDataSourceSchema := schema.Schema{
		Attributes: baseEdaDataSource.GetBaseAttributes(),
	}

	testCases := []struct {
		name                string
		ctx                 context.Context
		edaEndpoint         string
		expectedID          int64
		expectErrorLog      bool
		expectedLogMessage  string
		expectDiagnosticErr bool
		expectedDiagSummary string
		expectedDiagDetail  string
		request             datasource.ReadRequest
		response            *datasource.ReadResponse
	}{
		{
			name:                "Success scenario",
			ctx:                 t.Context(),
			edaEndpoint:         "/api/eda/v1",
			expectedID:          123,
			expectErrorLog:      false,
			expectDiagnosticErr: false,
			request: datasource.ReadRequest{
				Config: tfsdk.Config{
					Schema: baseEdaDataSourceSchema,
					Raw:    createTerraformValue(t.Context(), baseEdaDataSourceSchema, eventStreamName),
				},
			},
			response: &datasource.ReadResponse{
				State: tfsdk.State{
					Schema: baseEdaDataSourceSchema,
					Raw:    createTerraformValue(t.Context(), baseEdaDataSourceSchema, eventStreamName),
				},
			},
		},
		{
			name:                "Invalid EDA endpoint",
			ctx:                 t.Context(),
			edaEndpoint:         "",
			expectDiagnosticErr: true,
			expectedDiagSummary: "EDA API Endpoint is empty",
			expectedDiagDetail:  "Expected a valid endpoint but was an empty string. Please report this issue to the provider developers.",
			request: datasource.ReadRequest{
				Config: tfsdk.Config{
					Schema: baseEdaDataSourceSchema,
					Raw:    createTerraformValue(t.Context(), baseEdaDataSourceSchema, eventStreamName),
				},
			},
			response: &datasource.ReadResponse{
				State: tfsdk.State{
					Schema: baseEdaDataSourceSchema,
					Raw:    createTerraformValue(t.Context(), baseEdaDataSourceSchema, eventStreamName),
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			client := NewMockProviderHTTPClient(ctrl)
			client.EXPECT().getEdaApiEndpoint().Times(1).Return(tc.edaEndpoint)

			// Mock the HTTP GET call with expected parameters
			expectedParams := map[string]string{
				"name": eventStreamName,
			}

			// Mock API response
			mockResponse := fmt.Sprintf(`{
				"results": [
					{
						"id": %[1]d,
						"name": "%[2]s",
						"url": "/api/eda/v1/event-streams/%[1]d/"
					}
				]
			}`, tc.expectedID, eventStreamName)

			client.EXPECT().GetWithParams(gomock.Any(), expectedParams).AnyTimes().Return([]byte(mockResponse), diag.Diagnostics{})

			testDataSource := NewBaseEdaDataSource(client, StringDescriptions{
				ApiEntitySlug:         "event-streams", // This gets appended to the EDA endpoint
				DescriptiveEntityName: "Event Stream",
				MetadataEntitySlug:    "eventstream",
			})

			testDataSource.Read(tc.ctx, tc.request, tc.response)

			checkTerraformDiagnostics(t, tc.expectDiagnosticErr, getDiags(tc.response), tc.expectedDiagSummary, tc.expectedDiagDetail)

			// Verify the response state has the expected values
			var resultState BaseEdaSourceModel
			diags := tc.response.State.Get(tc.ctx, &resultState)
			if diags.HasError() {
				t.Fatalf("Failed to get state: %+v", diags)
			}

			// Verify the parsed values
			if resultState.ID.ValueInt64() != tc.expectedID {
				t.Errorf("Expected ID to be %d, got %d", tc.expectedID, resultState.ID.ValueInt64())
			}
			if resultState.Name.ValueString() != eventStreamName {
				t.Errorf("Expected name to be %s, got %s", eventStreamName, resultState.Name.ValueString())
			}
		})
	}
}

// TestBaseEdaSourceModelParseHttpResponse tests the ParseHttpResponse function handles various API response
// scenarios including valid responses, invalid JSON, and unexpected result counts.
func TestBaseEdaSourceModelParseHttpResponse(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name                string
		responseBody        []byte
		expectError         bool
		expectedDiagSummary string
		expectedDiagDetail  string
		expectedID          int64
		expectedName        string
		expectedURL         string
	}{
		{
			name: "Valid response with single result",
			responseBody: []byte(`{
				"results": [
					{
						"id": 123,
						"name": "test-eventstream",
						"url": "/api/eda/v1/event-streams/123/"
					}
				]
			}`),
			expectError:  false,
			expectedID:   123,
			expectedName: "test-eventstream",
			expectedURL:  "/api/eda/v1/event-streams/123/",
		},
		{
			name:                "Invalid JSON response",
			responseBody:        []byte(`{"invalid": json}`),
			expectError:         true,
			expectedDiagSummary: "Error parsing JSON response from AAP", // NOSONAR
			expectedDiagDetail:  "invalid character 'j' looking for beginning of value",
		},
		{
			name: "Empty results array",
			responseBody: []byte(`{
				"results": []
			}`),
			expectError:         true,
			expectedDiagSummary: "No event streams found in AAP", // NOSONAR
			expectedDiagDetail:  "Expected 1 object in JSON response, found 0",
		},
		{
			name: "Multiple results",
			responseBody: []byte(`{
				"results": [
					{
						"id": 123,
						"name": "test-eventstream-1",
						"url": "/api/eda/v1/event-streams/123/"
					},
					{
						"id": 456,
						"name": "test-eventstream-2",
						"url": "/api/eda/v1/event-streams/456/"
					}
				]
			}`),
			expectError:         true,
			expectedDiagSummary: "No event streams found in AAP",
			expectedDiagDetail:  "Expected 1 object in JSON response, found 2",
		},
		{
			name: "Valid response with null/empty values",
			responseBody: []byte(`{
				"results": [
					{
						"id": 0,
						"name": "",
						"url": ""
					}
				]
			}`),
			expectError:  false,
			expectedID:   0,
			expectedName: "",
			expectedURL:  "",
		},
		{
			name: "Valid response with missing fields",
			responseBody: []byte(`{
				"results": [
					{
						"id": 789
					}
				]
			}`),
			expectError:  false,
			expectedID:   789,
			expectedName: "",
			expectedURL:  "",
		},
		{
			name:                "Completely malformed JSON",
			responseBody:        []byte(`not json at all`),
			expectError:         true,
			expectedDiagSummary: "Error parsing JSON response from AAP",
			expectedDiagDetail:  "invalid character 'o' in literal null (expecting 'u')",
		},
		{
			name:                "Empty response body",
			responseBody:        []byte(``),
			expectError:         true,
			expectedDiagSummary: "Error parsing JSON response from AAP",
			expectedDiagDetail:  "unexpected end of JSON input",
		},
		{
			name: "Missing results field",
			responseBody: []byte(`{
				other_field": "value"
			}`),
			expectError:         true,
			expectedDiagSummary: "Error parsing JSON response from AAP",
			expectedDiagDetail:  "invalid character 'o' looking for beginning of object key string",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var model BaseEdaSourceModel
			diags := model.ParseHttpResponse(tc.responseBody)

			checkTerraformDiagnostics(t, tc.expectError, &diags, tc.expectedDiagSummary, tc.expectedDiagDetail)

			// Check parsed values for successful cases
			if !tc.expectError && model.ID.ValueInt64() != tc.expectedID {
				t.Errorf("Expected ID to be %d, got %d", tc.expectedID, model.ID.ValueInt64())
			}
			if !tc.expectError && model.Name.ValueString() != tc.expectedName {
				t.Errorf("Expected name to be '%s', got '%s'", tc.expectedName, model.Name.ValueString())
			}
			if !tc.expectError && model.URL.ValueString() != tc.expectedURL {
				t.Errorf("Expected URL to be '%s', got '%s'", tc.expectedURL, model.URL.ValueString())
			}
		})
	}
}

// checkTerraformLogs looks for a specific log message in a collection of log messages. The t.Error() function
// is called if the expected message is not found.
func checkTerraformLogs(t testing.TB, logs *bytes.Buffer, expectErrorLog bool, expectedLogMessage string) {
	t.Helper()

	if !expectErrorLog {
		return
	}

	found := false
	scanner := bufio.NewScanner(logs)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), expectedLogMessage) {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected error message containing '%s'", expectedLogMessage)
	}
}

// checkTerraformDiagnostics ensures that the expected diagnostics were emitted.
func checkTerraformDiagnostics(t testing.TB, expectDiagnosticErr bool, diags *diag.Diagnostics, expectedDiagSummary string, expectedDiagDetail string) {
	t.Helper()

	// Check for unexpected diagnostic errors.
	if !expectDiagnosticErr && diags.HasError() {
		t.Errorf("Unexpected diagnostic error: %+v", diags)
	}

	// Check for expected diagnostic errors but none seen.
	if expectDiagnosticErr && !diags.HasError() {
		t.Error("Expected diagnostic error but didn't get one")
	}

	// Check that the expected error appears in the diagnostic errors.
	if expectDiagnosticErr && diags.HasError() {
		expectedDiagErr := diag.NewErrorDiagnostic(expectedDiagSummary, expectedDiagDetail)
		if !diags.Errors().Contains(expectedDiagErr) {
			t.Errorf("Expected diagnostic error with summary=%s and detail=%s. Got %+v", expectedDiagSummary, expectedDiagDetail, diags)
		}
	}
}

// getDiags gets the Terraform diagnostics from a datasource response. An empty Diagnostics slice is
// returned if the response is nil or of the incorrect type.
func getDiags[T *datasource.ConfigureResponse | *datasource.ReadResponse](response T) *diag.Diagnostics {
	if response == nil {
		return &diag.Diagnostics{}
	}

	switch value := any(response).(type) {
	case *datasource.ConfigureResponse:
		return &value.Diagnostics
	case *datasource.ReadResponse:
		return &value.Diagnostics
	}

	return &diag.Diagnostics{}
}

// createTerraformValue creates a `tftypes.Value` to be used in constructing Terraform requests and responses.
func createTerraformValue(ctx context.Context, schema schema.Schema, nameValue string) tftypes.Value { //nolint:unparam
	return tftypes.NewValue(
		schema.Type().TerraformType(ctx),
		map[string]tftypes.Value{
			"name": tftypes.NewValue(tftypes.String, nameValue),
			"id":   tftypes.NewValue(tftypes.Number, nil),
			"url":  tftypes.NewValue(tftypes.String, nil),
		},
	)
}
