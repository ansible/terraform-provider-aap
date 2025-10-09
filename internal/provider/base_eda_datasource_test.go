package provider

import (
	"bufio"
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-log/tflogtest"
)

type configureTestCase struct {
	name                string
	ctx                 context.Context
	request             datasource.ConfigureRequest
	response            *datasource.ConfigureResponse
	expectErrorLog      bool
	expectedLogMessage  string
	expectClientSet     bool
	expectDiagnosticErr bool
	expectedDiagSummary string
	expectedDiagDetail  string
}

// TestBaseEDADataSourceMetadata tests that the Metadata function returns the correct metadata for the
// datasource. This means a string of the form providerName_datasourceName is returned.
func TestBaseEdaDataSourceMetadata(t *testing.T) {
	t.Parallel()

	testDataSource := NewBaseEdaDataSource(nil, StringDescriptions{
		ApiEntitySlug:         "datasourceApiSlug",
		DescriptiveEntityName: "datasourceDescriptiveName",
		MetadataEntitySlug:    "datasourceMetadataSlug",
	})

	ctx := context.Background()
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

	ctx := context.Background()
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

	cancelledContext, cancelFunc := context.WithCancel(context.Background())
	cancelFunc()

	testCases := []configureTestCase{
		{
			name: "Success scenario",
			ctx:  context.Background(),
			request: datasource.ConfigureRequest{
				ProviderData: &AAPClient{}, // Valid client
			},
			response:            &datasource.ConfigureResponse{},
			expectErrorLog:      false,
			expectDiagnosticErr: false,
			expectClientSet:     true,
		},
		{
			name:                "Response object is nil",
			ctx:                 context.Background(),
			request:             datasource.ConfigureRequest{},
			response:            nil,
			expectErrorLog:      true,
			expectedLogMessage:  "Response not defined, we cannot continue with the execution",
			expectClientSet:     false,
			expectDiagnosticErr: false,
		},
		{
			name:                "Context not active",
			ctx:                 cancelledContext,
			request:             datasource.ConfigureRequest{},
			response:            &datasource.ConfigureResponse{},
			expectErrorLog:      false,
			expectClientSet:     false,
			expectDiagnosticErr: true,
			expectedDiagSummary: "Aborting Configure operation",
			expectedDiagDetail:  "Context is not active, we cannot continue with the execution",
		},
		{
			name:                "ProviderData is nil",
			ctx:                 context.Background(),
			request:             datasource.ConfigureRequest{ProviderData: nil},
			response:            &datasource.ConfigureResponse{},
			expectErrorLog:      false,
			expectClientSet:     false,
			expectDiagnosticErr: false,
		},
		{
			name:                "Wrong ProviderData type",
			ctx:                 context.Background(),
			request:             datasource.ConfigureRequest{ProviderData: "wrong"},
			response:            &datasource.ConfigureResponse{},
			expectErrorLog:      false,
			expectClientSet:     false,
			expectDiagnosticErr: true,
			expectedDiagSummary: "Unexpected Data Source Configure Type",
			expectedDiagDetail:  "Expected *AAPClient, got: string. Please report this issue to the provider developers.",
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

			checkTerraformLogs(t, &tc, &logs)
			checkTerraformDiagnostics(t, &tc)

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

// checkTerraformLogs looks for a specific log message in a collection of log messages. The t.Error() function
// is called if the expected message is not found.
func checkTerraformLogs(t testing.TB, tc *configureTestCase, logs *bytes.Buffer) {
	t.Helper()

	if !tc.expectErrorLog {
		return
	}

	found := false
	scanner := bufio.NewScanner(logs)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), tc.expectedLogMessage) {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected error message containing '%s'", tc.expectedLogMessage)
	}
}

// checkTerraformDiagnostics ensures that the expected diagnostics were emitted.
func checkTerraformDiagnostics(t testing.TB, tc *configureTestCase) {
	t.Helper()

	// Check diagnostics for error conditions
	if tc.expectDiagnosticErr && !tc.response.Diagnostics.HasError() {
		t.Error("Expected diagnostic error but didn't get one")
	}
	if tc.expectDiagnosticErr && tc.response.Diagnostics.HasError() {
		expectedDiagErr := diag.NewErrorDiagnostic(tc.expectedDiagSummary, tc.expectedDiagDetail)
		if !tc.response.Diagnostics.Errors().Contains(expectedDiagErr) {
			t.Errorf("Expected diagnostic error with summary=%s and detail=%s. Got %+v", tc.expectedDiagSummary, tc.expectedDiagDetail, tc.response.Diagnostics)
		}
	}
}
