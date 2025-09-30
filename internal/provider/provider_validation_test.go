// provider_validation_test.go contains tests for provider configuration validation and error handling.
// This includes unknown value detection, error message validation, and validation logic.
package provider

import (
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestCheckUnknownValue(t *testing.T) {
	testTable := []struct {
		model        aapProviderModel
		name         string
		expectError  bool
		errorSummary string
		errorDetail  string
	}{
		{
			name: "no errors with nothing unknown (token)",
			model: aapProviderModel{
				Host:               types.StringValue("http://localhost"),
				Token:              types.StringValue("test-token"),
				InsecureSkipVerify: types.BoolValue(true),
				Timeout:            types.Int64Value(30),
			},
			expectError:  false,
			errorSummary: "",
			errorDetail:  "",
		},
		{
			name: "no errors with nothing unknown (basic)",
			model: aapProviderModel{
				Host:               types.StringValue("http://localhost"),
				Username:           types.StringValue("username"),
				Password:           types.StringValue("password"),
				InsecureSkipVerify: types.BoolValue(true),
				Timeout:            types.Int64Value(30),
			},
			expectError:  false,
			errorSummary: "",
			errorDetail:  "",
		},
		{
			name: "unknown host",
			model: aapProviderModel{
				Host:               types.StringUnknown(),
				Username:           types.StringValue("username"),
				Password:           types.StringValue("password"),
				InsecureSkipVerify: types.BoolValue(true),
				Timeout:            types.Int64Value(30),
			},
			expectError:  true,
			errorSummary: "Unknown AAP API host",
			errorDetail:  "AAP_HOSTNAME",
		},
		{
			name: "unknown username",
			model: aapProviderModel{
				Host:               types.StringValue("http://localhost"),
				Username:           types.StringUnknown(),
				Password:           types.StringValue("password"),
				InsecureSkipVerify: types.BoolValue(true),
				Timeout:            types.Int64Value(30),
			},
			expectError:  true,
			errorSummary: "Unknown AAP API username",
			errorDetail:  "AAP_USERNAME",
		},
		{
			name: "unknown password",
			model: aapProviderModel{
				Host:               types.StringValue("http://localhost"),
				Username:           types.StringValue("username"),
				Password:           types.StringUnknown(),
				InsecureSkipVerify: types.BoolValue(true),
				Timeout:            types.Int64Value(30),
			},
			expectError:  true,
			errorSummary: "Unknown AAP API password",
			errorDetail:  "AAP_PASSWORD",
		},
		{
			name: "unknown token",
			model: aapProviderModel{
				Host:               types.StringValue("http://localhost"),
				Token:              types.StringUnknown(),
				InsecureSkipVerify: types.BoolValue(true),
				Timeout:            types.Int64Value(30),
			},
			expectError:  true,
			errorSummary: "Unknown AAP API token",
			errorDetail:  "AAP_TOKEN",
		},
		{
			name: "unknown insecure skip verify",
			model: aapProviderModel{
				Host:               types.StringValue("http://localhost"),
				Username:           types.StringValue("username"),
				Password:           types.StringValue("password"),
				InsecureSkipVerify: types.BoolUnknown(),
				Timeout:            types.Int64Value(30),
			},
			expectError:  true,
			errorSummary: "Unknown AAP API insecure_skip_verify",
			errorDetail:  "AAP_INSECURE_SKIP_VERIFY",
		},
		{
			name: "unknown timeout",
			model: aapProviderModel{
				Host:               types.StringValue("http://localhost"),
				Username:           types.StringValue("username"),
				Password:           types.StringValue("password"),
				InsecureSkipVerify: types.BoolValue(true),
				Timeout:            types.Int64Unknown(),
			},
			expectError:  true,
			errorSummary: "Unknown AAP API timeout",
			errorDetail:  "AAP_TIMEOUT",
		},
	}

	for _, tc := range testTable {
		t.Run(tc.name, func(t *testing.T) {
			diags := diag.Diagnostics{}
			tc.model.checkUnknownValue(&diags)
			actualError := diags.HasError()
			if actualError != tc.expectError {
				t.Errorf("Expected errors '%v', actual '%v'", tc.expectError, actualError)
			}
			found := false
			for _, err := range diags.Errors() {
				if strings.Contains(err.Summary(), tc.errorSummary) &&
					strings.Contains(err.Detail(), tc.errorDetail) {
					found = true
				}
			}
			if !found && tc.expectError {
				t.Errorf("Did not find error with expected summary '%v', detail containing '%v'. Actual errors %v",
					tc.errorSummary, tc.errorDetail, diags.Errors())
			}
		})
	}
}

// TestCheckUnknownValueMultiple tests the behavior when multiple provider values are unknown.
// This complements TestCheckUnknownValue which tests individual cases with detailed error message validation.
func TestCheckUnknownValueMultiple(t *testing.T) {
	testCases := []struct {
		name           string
		config         aapProviderModel
		expectedErrors int
		expectedFields []string
	}{
		{
			name: "no unknown values",
			config: aapProviderModel{
				Host:               types.StringValue("https://localhost"),
				Username:           types.StringValue("user"),
				Password:           types.StringValue("pass"),
				InsecureSkipVerify: types.BoolValue(true),
				Timeout:            types.Int64Value(10),
			},
			expectedErrors: 0,
			expectedFields: []string{},
		},
		{
			name: "all unknown values",
			config: aapProviderModel{
				Host:               types.StringUnknown(),
				Username:           types.StringUnknown(),
				Password:           types.StringUnknown(),
				InsecureSkipVerify: types.BoolUnknown(),
				Timeout:            types.Int64Unknown(),
			},
			expectedErrors: 5,
			expectedFields: []string{"host", "username", "password", "insecure_skip_verify", "timeout"},
		},
		{
			name: "single unknown value",
			config: aapProviderModel{
				Host:               types.StringUnknown(),
				Username:           types.StringValue("user"),
				Password:           types.StringValue("pass"),
				InsecureSkipVerify: types.BoolValue(true),
				Timeout:            types.Int64Value(10),
			},
			expectedErrors: 1,
			expectedFields: []string{"host"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp := &provider.ConfigureResponse{}

			tc.config.checkUnknownValue(&resp.Diagnostics)

			if resp.Diagnostics.ErrorsCount() != tc.expectedErrors {
				t.Errorf("Expected %d errors, got %d", tc.expectedErrors, resp.Diagnostics.ErrorsCount())
			}

			// Check that expected fields are mentioned in error messages
			if tc.expectedErrors > 0 {
				errors := resp.Diagnostics.Errors()
				for _, expectedField := range tc.expectedFields {
					found := false
					for _, err := range errors {
						if strings.Contains(err.Detail(), expectedField) {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected error message to contain field %s", expectedField)
					}
				}
			}
		})
	}
}
