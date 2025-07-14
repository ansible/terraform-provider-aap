package provider

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

const providerName = "aap"

// testAccProtoV6ProviderFactories are used to instantiate a provider during
// acceptance testing. The factory function will be invoked for every Terraform
// CLI command executed to create a provider server to which the CLI can
// reattach.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	providerName: providerserver.NewProtocol6WithError(New("test")()),
}

func testAccPreCheck(t *testing.T) {
	requiredAAPEnvVars := map[string]string{
		"AAP_HOST":                 "https://localhost:8043",
		"AAP_USERNAME":             "",
		"AAP_PASSWORD":             "",
		"AAP_INSECURE_SKIP_VERIFY": "true",
	}

	for k, d := range requiredAAPEnvVars {
		v := os.Getenv(k)
		if v == "" {
			if d == "" {
				t.Fatalf("'%s' environment variable must be set for acceptance tests", k)
			} else {
				t.Setenv(k, d)
			}
		}
	}
}

func testMethodResource(method string, urlPath string) ([]byte, error) {
	host := os.Getenv("AAP_HOST")
	username := os.Getenv("AAP_USERNAME")
	password := os.Getenv("AAP_PASSWORD")

	client, diags := NewClient(host, &username, &password, true, 0)
	if diags.HasError() {
		return nil, fmt.Errorf("%v", diags.Errors())
	}

	var body []byte
	switch method {
	case http.MethodGet:
		body, diags = client.Get(urlPath)
	case http.MethodDelete:
		body, diags = client.Delete(urlPath)
	}

	if diags.HasError() {
		return nil, fmt.Errorf("%v", diags.Errors())
	}

	return body, nil
}

func testGetResource(urlPath string) ([]byte, error) {
	return testMethodResource(http.MethodGet, urlPath)
}

func testDeleteResource(urlPath string) ([]byte, error) {
	return testMethodResource(http.MethodDelete, urlPath)
}

func TestReadValues(t *testing.T) {
	testTable := []struct {
		name               string
		config             aapProviderModel
		envVars            map[string]string
		Host               string
		Username           string
		Password           string
		InsecureSkipVerify bool
		Timeout            int64
		Errors             int
	}{
		{
			name:               "No defined values",
			config:             aapProviderModel{},
			envVars:            map[string]string{},
			Host:               "",
			Username:           "",
			Password:           "",
			InsecureSkipVerify: DefaultInsecureSkipVerify,
			Timeout:            DefaultTimeOut,
			Errors:             0,
		},
		{
			name:   "Using env variables only",
			config: aapProviderModel{},
			envVars: map[string]string{
				"AAP_HOST":                 "https://172.0.0.1:9000",
				"AAP_USERNAME":             "user988",
				"AAP_PASSWORD":             "@pass123#",
				"AAP_INSECURE_SKIP_VERIFY": "true",
				"AAP_TIMEOUT":              "30",
			},
			Host:               "https://172.0.0.1:9000",
			Username:           "user988",
			Password:           "@pass123#",
			InsecureSkipVerify: true,
			Timeout:            30,
			Errors:             0,
		},
		{
			name: "Using both configuration and envs value",
			config: aapProviderModel{
				Host:               types.StringValue("https://172.0.0.1:9000"),
				Username:           types.StringValue("user988"),
				Password:           types.StringValue("@pass123#"),
				InsecureSkipVerify: types.BoolValue(true),
				Timeout:            types.Int64Value(30),
			},
			envVars: map[string]string{
				"AAP_HOST":                 "https://168.3.5.11:8043",
				"AAP_USERNAME":             "ansible",
				"AAP_PASSWORD":             "testing#$%",
				"AAP_INSECURE_SKIP_VERIFY": "false",
				"AAP_TIMEOUT":              "3",
			},
			Host:               "https://172.0.0.1:9000",
			Username:           "user988",
			Password:           "@pass123#",
			InsecureSkipVerify: true,
			Timeout:            30,
			Errors:             0,
		},
		{
			name: "Using configuration value",
			config: aapProviderModel{
				Host:               types.StringValue("https://172.0.0.1:9000"),
				Username:           types.StringValue("user988"),
				Password:           types.StringValue("@pass123#"),
				InsecureSkipVerify: types.BoolValue(true),
				Timeout:            types.Int64Value(30),
			},
			envVars:            map[string]string{},
			Host:               "https://172.0.0.1:9000",
			Username:           "user988",
			Password:           "@pass123#",
			InsecureSkipVerify: true,
			Timeout:            30,
			Errors:             0,
		},
		{
			name:   "Bad value for env variable",
			config: aapProviderModel{},
			envVars: map[string]string{
				"AAP_INSECURE_SKIP_VERIFY": "falsy",
				"AAP_TIMEOUT":              "null",
			},
			Errors: 2,
		},
		{
			name: "Using null values in configuration",
			config: aapProviderModel{
				Host:               types.StringNull(),
				Username:           types.StringNull(),
				Password:           types.StringNull(),
				InsecureSkipVerify: types.BoolNull(),
				Timeout:            types.Int64Null(),
			},
			envVars:            map[string]string{},
			Host:               "",
			Username:           "",
			Password:           "",
			InsecureSkipVerify: DefaultInsecureSkipVerify,
			Timeout:            DefaultTimeOut,
			Errors:             0,
		},
	}
	var providerEnvVars = []string{
		"AAP_HOST",
		"AAP_USERNAME",
		"AAP_PASSWORD",
		"AAP_INSECURE_SKIP_VERIFY",
		"AAP_TIMEOUT",
	}
	for _, tc := range testTable {
		t.Run(tc.name, func(t *testing.T) {
			var host, username, password string
			var insecureSkipVerify bool
			var timeout int64
			var resp provider.ConfigureResponse
			// Set env variables
			for _, name := range providerEnvVars {
				if value, ok := tc.envVars[name]; ok {
					t.Setenv(name, value)
				} else {
					t.Setenv(name, "")
				}
			}
			// ReadValues()
			tc.config.ReadValues(&host, &username, &password, &insecureSkipVerify, &timeout, &resp)
			if tc.Errors != resp.Diagnostics.ErrorsCount() {
				t.Errorf("Errors count expected=(%d) - found=(%d)", tc.Errors, resp.Diagnostics.ErrorsCount())
			} else if tc.Errors == 0 {
				if host != tc.Host {
					t.Errorf("Host values differ expected=(%s) - computed=(%s)", tc.Host, host)
				}
				if username != tc.Username {
					t.Errorf("Username values differ expected=(%s) - computed=(%s)", tc.Username, username)
				}
				if password != tc.Password {
					t.Errorf("Password values differ expected=(%s) - computed=(%s)", tc.Password, password)
				}
				if insecureSkipVerify != tc.InsecureSkipVerify {
					t.Errorf("InsecureSkipVerify values differ expected=(%v) - computed=(%v)", tc.InsecureSkipVerify, insecureSkipVerify)
				}
				if timeout != tc.Timeout {
					t.Errorf("Timeout values differ expected=(%d) - computed=(%d)", tc.Timeout, timeout)
				}
			}
		})
	}
}

func TestNew(t *testing.T) {
	version := "1.0.0"
	factory := New(version)

	// Test that the factory returns a provider function
	if factory == nil {
		t.Fatal("New() should return a provider factory function")
	}

	// Test that the factory function returns a provider
	provider := factory()
	if provider == nil {
		t.Fatal("Provider factory should return a provider instance")
	}

	// Test that the provider is of the correct type
	aapProvider, ok := provider.(*aapProvider)
	if !ok {
		t.Fatal("Provider should be of type *aapProvider")
	}

	// Test that the version is set correctly
	if aapProvider.version != version {
		t.Errorf("Expected version %s, got %s", version, aapProvider.version)
	}
}

func TestMetadata(t *testing.T) {
	testCases := []struct {
		name         string
		version      string
		expectedType string
	}{
		{
			name:         "test version",
			version:      "test",
			expectedType: "aap",
		},
		{
			name:         "dev version",
			version:      "dev",
			expectedType: "aap",
		},
		{
			name:         "release version",
			version:      "1.0.0",
			expectedType: "aap",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			p := &aapProvider{version: tc.version}
			req := provider.MetadataRequest{}
			resp := &provider.MetadataResponse{}

			p.Metadata(context.TODO(), req, resp)

			if resp.TypeName != tc.expectedType {
				t.Errorf("Expected TypeName %s, got %s", tc.expectedType, resp.TypeName)
			}

			if resp.Version != tc.version {
				t.Errorf("Expected Version %s, got %s", tc.version, resp.Version)
			}
		})
	}
}

func TestSchema(t *testing.T) {
	p := &aapProvider{}
	req := provider.SchemaRequest{}
	resp := &provider.SchemaResponse{}

	p.Schema(context.TODO(), req, resp)

	// Check that schema is not nil
	if resp.Schema.Attributes == nil {
		t.Fatal("Schema attributes should not be nil")
	}

	// Check that required attributes are present
	expectedAttrs := []string{"host", "username", "password", "insecure_skip_verify", "timeout"}
	for _, attr := range expectedAttrs {
		if _, exists := resp.Schema.Attributes[attr]; !exists {
			t.Errorf("Expected attribute %s to be present in schema", attr)
		}
	}

	// Check that password is marked as sensitive
	if passwordAttr, exists := resp.Schema.Attributes["password"]; exists {
		if stringAttr, ok := passwordAttr.(schema.StringAttribute); ok {
			if !stringAttr.Sensitive {
				t.Error("Password attribute should be marked as sensitive")
			}
		}
	}

	// Check that timeout has description
	if timeoutAttr, exists := resp.Schema.Attributes["timeout"]; exists {
		if int64Attr, ok := timeoutAttr.(schema.Int64Attribute); ok {
			if int64Attr.Description == "" {
				t.Error("Timeout attribute should have a description")
			}
		}
	}
}

func TestAddConfigurationAttributeError(t *testing.T) {
	testCases := []struct {
		name               string
		attrName           string
		envName            string
		isUnknown          bool
		expectUnknownError bool
	}{
		{
			name:               "unknown value error",
			attrName:           "host",
			envName:            "AAP_HOST",
			isUnknown:          true,
			expectUnknownError: true,
		},
		{
			name:               "missing value error",
			attrName:           "username",
			envName:            "AAP_USERNAME",
			isUnknown:          false,
			expectUnknownError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp := &provider.ConfigureResponse{}

			AddConfigurationAttributeError(resp, tc.attrName, tc.envName, tc.isUnknown)

			if resp.Diagnostics.ErrorsCount() != 1 {
				t.Errorf("Expected 1 error, got %d", resp.Diagnostics.ErrorsCount())
			}

			errors := resp.Diagnostics.Errors()
			if len(errors) > 0 {
				errorMsg := errors[0].Detail()
				if tc.expectUnknownError {
					if !contains(errorMsg, "unknown configuration value") {
						t.Error("Expected unknown configuration value error message")
					}
				} else {
					if !contains(errorMsg, "missing or empty value") {
						t.Error("Expected missing or empty value error message")
					}
				}

				// Check that environment variable name is mentioned
				if !contains(errorMsg, tc.envName) {
					t.Errorf("Expected error message to contain environment variable name %s", tc.envName)
				}
			}
		})
	}
}

func TestDataSources(t *testing.T) {
	p := &aapProvider{}

	dataSources := p.DataSources(context.TODO())

	expectedDataSources := []string{
		"NewInventoryDataSource",
		"NewJobTemplateDataSource",
		"NewWorkflowJobTemplateDataSource",
		"NewOrganizationDataSource",
	}

	if len(dataSources) != len(expectedDataSources) {
		t.Errorf("Expected %d data sources, got %d", len(expectedDataSources), len(dataSources))
	}

	// Test that each data source factory function returns a valid data source
	for i, factory := range dataSources {
		if factory == nil {
			t.Errorf("Data source factory at index %d should not be nil", i)
			continue
		}

		dataSource := factory()
		if dataSource == nil {
			t.Errorf("Data source factory at index %d should return a valid data source", i)
		}
	}
}

func TestResources(t *testing.T) {
	p := &aapProvider{}

	resources := p.Resources(context.TODO())

	expectedResources := []string{
		"NewInventoryResource",
		"NewJobResource",
		"NewWorkflowJobResource",
		"NewGroupResource",
		"NewHostResource",
	}

	if len(resources) != len(expectedResources) {
		t.Errorf("Expected %d resources, got %d", len(expectedResources), len(resources))
	}

	// Test that each resource factory function returns a valid resource
	for i, factory := range resources {
		if factory == nil {
			t.Errorf("Resource factory at index %d should not be nil", i)
			continue
		}

		resource := factory()
		if resource == nil {
			t.Errorf("Resource factory at index %d should return a valid resource", i)
		}
	}
}

func TestCheckUnknownValue(t *testing.T) {
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

			tc.config.checkUnknownValue(resp)

			if resp.Diagnostics.ErrorsCount() != tc.expectedErrors {
				t.Errorf("Expected %d errors, got %d", tc.expectedErrors, resp.Diagnostics.ErrorsCount())
			}

			// Check that expected fields are mentioned in error messages
			if tc.expectedErrors > 0 {
				errors := resp.Diagnostics.Errors()
				for _, expectedField := range tc.expectedFields {
					found := false
					for _, err := range errors {
						if contains(err.Detail(), expectedField) {
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

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr ||
		len(s) > len(substr) && s[len(s)-len(substr):] == substr ||
		len(s) > len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
