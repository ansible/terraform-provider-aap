package provider

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

const providerName = "aap"

// testAccProtoV6ProviderFactories are used to instantiate a provider during
// acceptance testing. The factory function will be invoked for every Terraform
// CLI command executed to create a provider server to which the CLI can
// reattach.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	providerName: providerserver.NewProtocol6WithError(New("test")()),
}

// testAccPreCheck is used by acceptance tests in the PreCheck block.
func testAccPreCheck(t *testing.T) {
	requiredAAPEnvVars := map[string]string{
		"AAP_HOSTNAME":             "https://localhost:8043",
		"AAP_USERNAME":             "",
		"AAP_PASSWORD":             "",
		"AAP_INSECURE_SKIP_VERIFY": "true",
	}

	_, tokenSet := os.LookupEnv("AAP_TOKEN")
	if tokenSet {
		t.Log("'AAP_TOKEN' is set, using token authentication in acceptance tests")
		// Token is set in environment, use that
		delete(requiredAAPEnvVars, "AAP_USERNAME")
		delete(requiredAAPEnvVars, "AAP_PASSWORD")
		requiredAAPEnvVars["AAP_TOKEN"] = ""
	} else {
		t.Log("'AAP_TOKEN' is not set, using basic authentication in acceptance tests")
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
	// Prefer AAP_HOSTNAME, fallback to AAP_HOST
	host := os.Getenv("AAP_HOSTNAME")
	if host == "" {
		host = os.Getenv("AAP_HOST")
	}

	// Prefer AAP_TOKEN, fallback to AAP_USERNAME / AAP_PASSWORD
	token := os.Getenv("AAP_TOKEN")
	username := os.Getenv("AAP_USERNAME")
	password := os.Getenv("AAP_PASSWORD")
	var authenticator AAPClientAuthenticator
	var diags diag.Diagnostics
	if len(token) > 0 {
		authenticator, diags = NewTokenAuthenticator(&token)
	} else {
		authenticator, diags = NewBasicAuthenticator(&username, &password)
	}
	if diags.HasError() {
		return nil, fmt.Errorf("%v", diags.Errors())
	}
	client, diags := NewClient(host, authenticator, true, 0)
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
		Token              string
		InsecureSkipVerify bool
		Timeout            int64
		Errors             int
		Warnings           int
	}{
		{
			name:               "No defined values",
			config:             aapProviderModel{},
			envVars:            map[string]string{},
			Host:               "",
			Username:           "",
			Password:           "",
			Token:              "",
			InsecureSkipVerify: DefaultInsecureSkipVerify,
			Timeout:            DefaultTimeOut,
			Errors:             0,
		},
		{
			name:   "Using env variables only, with token",
			config: aapProviderModel{},
			envVars: map[string]string{
				"AAP_HOSTNAME":             "https://172.0.0.1:9000",
				"AAP_TOKEN":                "test-token",
				"AAP_INSECURE_SKIP_VERIFY": "true",
				"AAP_TIMEOUT":              "30",
			},
			Host:               "https://172.0.0.1:9000",
			Token:              "test-token",
			InsecureSkipVerify: true,
			Timeout:            30,
			Errors:             0,
		},
		{
			name:   "Using env variables only, with username/password",
			config: aapProviderModel{},
			envVars: map[string]string{
				"AAP_HOSTNAME":             "https://172.0.0.1:9000",
				"AAP_USERNAME":             "user988",
				"AAP_PASSWORD":             "@pass123#",
				"AAP_INSECURE_SKIP_VERIFY": "true",
				"AAP_TIMEOUT":              "30",
			},
			Host:               "https://172.0.0.1:9000",
			Username:           "user988",
			Password:           "@pass123#",
			Token:              "",
			InsecureSkipVerify: true,
			Timeout:            30,
			Errors:             0,
		},
		{
			name:   "Using env variables only, legacy AAP_HOST",
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
				"AAP_HOSTNAME":             "https://168.3.5.11:8043",
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
		{
			name:   "Using env variables for configuration, ignores username/password when token is set",
			config: aapProviderModel{},
			envVars: map[string]string{
				"AAP_HOSTNAME": "https://172.0.0.1:9000",
				"AAP_USERNAME": "ansible",
				"AAP_PASSWORD": "testing#$%",
				"AAP_TOKEN":    "test-token",
			},
			Host:               "https://172.0.0.1:9000",
			Username:           "",
			Password:           "",
			Token:              "test-token",
			InsecureSkipVerify: false,
			Timeout:            5,
			Errors:             0,
		},
		{
			name: "Using configuration, ignores username/password when token is set and reports warnings",
			config: aapProviderModel{
				Host:     types.StringValue("https://172.0.0.1:9000"),
				Username: types.StringValue("user988"),
				Password: types.StringValue("@pass123#"),
				Token:    types.StringValue("test-token"),
			},
			envVars:            map[string]string{},
			Host:               "https://172.0.0.1:9000",
			Username:           "",
			Password:           "",
			Token:              "test-token",
			InsecureSkipVerify: false,
			Timeout:            5,
			Errors:             0,
			Warnings:           2,
		},
	}
	var providerEnvVars = []string{
		"AAP_HOSTNAME",
		"AAP_HOST",
		"AAP_USERNAME",
		"AAP_TOKEN",
		"AAP_PASSWORD",
		"AAP_INSECURE_SKIP_VERIFY",
		"AAP_TIMEOUT",
	}
	for _, tc := range testTable {
		t.Run(tc.name, func(t *testing.T) {
			var host, username, password, token string
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
			tc.config.ReadValues(&host, &username, &password, &token, &insecureSkipVerify, &timeout, &resp)
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
				if token != tc.Token {
					t.Errorf("Token values differ expected=(%s) - computed=(%s)", tc.Token, token)
				}
				if insecureSkipVerify != tc.InsecureSkipVerify {
					t.Errorf("InsecureSkipVerify values differ expected=(%v) - computed=(%v)", tc.InsecureSkipVerify, insecureSkipVerify)
				}
				if timeout != tc.Timeout {
					t.Errorf("Timeout values differ expected=(%d) - computed=(%d)", tc.Timeout, timeout)
				}
			}
			if tc.Warnings != resp.Diagnostics.WarningsCount() {
				t.Errorf("Warnings count expected=(%d) - found=(%d)", tc.Warnings, resp.Diagnostics.WarningsCount())
			}
		})
	}
}

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

func TestConfigure(t *testing.T) {
	testTable := []struct {
		name         string
		configValues map[string]tftypes.Value
		envVars      map[string]string
		expectErrors int
		errorSummary string
		errorDetail  string
	}{
		{
			name: "Missing host",
			configValues: map[string]tftypes.Value{
				"host":                 tftypes.NewValue(tftypes.String, ""),
				"username":             tftypes.NewValue(tftypes.String, "username"),
				"password":             tftypes.NewValue(tftypes.String, "password"),
				"token":                tftypes.NewValue(tftypes.String, ""),
				"insecure_skip_verify": tftypes.NewValue(tftypes.Bool, false),
				"timeout":              tftypes.NewValue(tftypes.Number, 30),
			},
			expectErrors: 1,
			errorSummary: "Missing AAP API host",
			errorDetail:  "AAP_HOSTNAME",
		},
		{
			name: "Missing token",
			configValues: map[string]tftypes.Value{
				"host":                 tftypes.NewValue(tftypes.String, "http://localhost"),
				"username":             tftypes.NewValue(tftypes.String, ""),
				"password":             tftypes.NewValue(tftypes.String, ""),
				"token":                tftypes.NewValue(tftypes.String, ""),
				"insecure_skip_verify": tftypes.NewValue(tftypes.Bool, false),
				"timeout":              tftypes.NewValue(tftypes.Number, 30),
			},
			expectErrors: 3,
			errorSummary: "Missing AAP API token",
			errorDetail:  "AAP_TOKEN",
		},
		{
			name: "Missing username",
			configValues: map[string]tftypes.Value{
				"host":                 tftypes.NewValue(tftypes.String, "http://localhost"),
				"username":             tftypes.NewValue(tftypes.String, ""),
				"password":             tftypes.NewValue(tftypes.String, "password"),
				"token":                tftypes.NewValue(tftypes.String, ""),
				"insecure_skip_verify": tftypes.NewValue(tftypes.Bool, false),
				"timeout":              tftypes.NewValue(tftypes.Number, 30),
			},
			expectErrors: 1,
			errorSummary: "Missing AAP API username",
			errorDetail:  "AAP_USERNAME",
		},
		{
			name: "Missing password",
			configValues: map[string]tftypes.Value{
				"host":                 tftypes.NewValue(tftypes.String, "http://localhost"),
				"username":             tftypes.NewValue(tftypes.String, "username"),
				"password":             tftypes.NewValue(tftypes.String, ""),
				"token":                tftypes.NewValue(tftypes.String, ""),
				"insecure_skip_verify": tftypes.NewValue(tftypes.Bool, false),
				"timeout":              tftypes.NewValue(tftypes.Number, 30),
			},
			expectErrors: 1,
			errorSummary: "Missing AAP API password",
			errorDetail:  "AAP_PASSWORD",
		},
	}

	for _, tc := range testTable {
		t.Run(tc.name, func(t *testing.T) {
			p := aapProvider{
				version: "test",
			}

			// To test aapProvider.Configure, we need a tfdsk.Config struct, which has a value and a schema
			var schemaResp provider.SchemaResponse
			p.Schema(context.TODO(), provider.SchemaRequest{}, &schemaResp)

			// Create a config value using the schema
			configValue := tftypes.NewValue(tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"host":                 tftypes.String,
					"username":             tftypes.String,
					"password":             tftypes.String,
					"token":                tftypes.String,
					"insecure_skip_verify": tftypes.Bool,
					"timeout":              tftypes.Number,
				},
			}, tc.configValues)

			// Create config using the helper
			config := tfsdk.Config{
				Raw:    configValue,
				Schema: schemaResp.Schema,
			}

			request := provider.ConfigureRequest{
				Config: config,
			}
			response := provider.ConfigureResponse{}

			p.Configure(context.TODO(), request, &response)

			actualErrors := response.Diagnostics.ErrorsCount()
			if actualErrors != tc.expectErrors {
				t.Errorf("Expected '%v' errors, actual count '%v'", tc.expectErrors, actualErrors)
			}
			found := false
			for _, err := range response.Diagnostics.Errors() {
				if strings.Contains(err.Summary(), tc.errorSummary) &&
					strings.Contains(err.Detail(), tc.errorDetail) {
					found = true
				}
			}
			if !found && tc.expectErrors > 0 {
				t.Errorf("Did not find error with expected summary '%v', detail containing '%v'. Actual errors %v",
					tc.errorSummary, tc.errorDetail, response.Diagnostics.Errors())
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
