// provider_test.go contains test infrastructure and structural tests for the AAP Terraform provider.
// This includes provider factory testing, metadata validation, schema verification, and registration testing.
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
			if int64Attr.MarkdownDescription == "" {
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

			AddConfigurationAttributeError(&resp.Diagnostics, tc.attrName, tc.envName, tc.isUnknown)

			if resp.Diagnostics.ErrorsCount() != 1 {
				t.Errorf("Expected 1 error, got %d", resp.Diagnostics.ErrorsCount())
			}

			errors := resp.Diagnostics.Errors()
			if len(errors) > 0 {
				errorMsg := errors[0].Detail()
				if tc.expectUnknownError {
					if !strings.Contains(errorMsg, "unknown configuration value") {
						t.Error("Expected unknown configuration value error message")
					}
				} else {
					if !strings.Contains(errorMsg, "missing or empty value") {
						t.Error("Expected missing or empty value error message")
					}
				}

				// Check that environment variable name is mentioned
				if !strings.Contains(errorMsg, tc.envName) {
					t.Errorf("Expected error message to contain environment variable name %s", tc.envName)
				}
			}
		})
	}
}

func TestDataSources(t *testing.T) {
	p := &aapProvider{}

	dataSources := p.DataSources(context.TODO())

	// Ensure we have at least some data sources defined
	if len(dataSources) == 0 {
		t.Fatal("Provider should define at least one data source")
	}

	// Test that each data source factory function is valid and returns a valid data source
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

	// Log the actual count for debugging purposes
	t.Logf("Provider defines %d data sources", len(dataSources))
}

func TestResources(t *testing.T) {
	p := &aapProvider{}

	resources := p.Resources(context.TODO())

	// Ensure we have at least some resources defined
	if len(resources) == 0 {
		t.Fatal("Provider should define at least one resource")
	}

	// Test that each resource factory function is valid and returns a valid resource
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

	// Log the actual count for debugging purposes
	t.Logf("Provider defines %d resources", len(resources))
}
