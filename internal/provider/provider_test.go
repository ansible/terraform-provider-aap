package provider

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/provider"
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

func testAccPreCheck(t *testing.T) {
	requiredAAPEnvVars := map[string]string{
		"AAP_HOSTNAME":             "https://localhost:8043",
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
	// Prefer AAP_HOSTNAME, fallback to AAP_HOST
	host := os.Getenv("AAP_HOSTNAME")
	if host == "" {
		host = os.Getenv("AAP_HOST")
	}
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
				"AAP_HOSTNAME":             "https://172.0.0.1:9000",
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
	}
	var providerEnvVars = []string{
		"AAP_HOSTNAME",
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

func TestCheckUnknownValue(t *testing.T) {
	testTable := []struct {
		model        aapProviderModel
		name         string
		expectError  bool
		errorSummary string
		errorDetail  string
	}{
		{
			name: "no errors with nothing unknown",
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
			response := provider.ConfigureResponse{}
			tc.model.checkUnknownValue(&response)
			actualError := response.Diagnostics.HasError()
			if actualError != tc.expectError {
				t.Errorf("Expected errors '%v', actual '%v'", tc.expectError, actualError)
			}
			found := false
			for _, err := range response.Diagnostics.Errors() {
				if strings.Contains(err.Summary(), tc.errorSummary) &&
					strings.Contains(err.Detail(), tc.errorDetail) {
					found = true
				}
			}
			if !found && tc.expectError {
				t.Errorf("Did not find error with expected summary '%v', detail containing '%v'. Actual errors %v",
					tc.errorSummary, tc.errorDetail, response.Diagnostics.Errors())
			}
		})
	}
}

func TestConfigure(t *testing.T) {
	testTable := []struct {
		name         string
		configValues map[string]tftypes.Value
		envVars      map[string]string
		expectError  bool
		errorSummary string
		errorDetail  string
	}{
		{
			name: "Missing host",
			configValues: map[string]tftypes.Value{
				"host":                 tftypes.NewValue(tftypes.String, ""),
				"username":             tftypes.NewValue(tftypes.String, "username"),
				"password":             tftypes.NewValue(tftypes.String, "password"),
				"insecure_skip_verify": tftypes.NewValue(tftypes.Bool, false),
				"timeout":              tftypes.NewValue(tftypes.Number, 30),
			},
			expectError:  true,
			errorSummary: "Missing AAP API host",
			errorDetail:  "AAP_HOSTNAME",
		},
		{
			name: "Missing username",
			configValues: map[string]tftypes.Value{
				"host":                 tftypes.NewValue(tftypes.String, "http://localhost"),
				"username":             tftypes.NewValue(tftypes.String, ""),
				"password":             tftypes.NewValue(tftypes.String, "password"),
				"insecure_skip_verify": tftypes.NewValue(tftypes.Bool, false),
				"timeout":              tftypes.NewValue(tftypes.Number, 30),
			},
			expectError:  true,
			errorSummary: "Missing AAP API username",
			errorDetail:  "AAP_USERNAME",
		},
		{
			name: "Missing password",
			configValues: map[string]tftypes.Value{
				"host":                 tftypes.NewValue(tftypes.String, "http://localhost"),
				"username":             tftypes.NewValue(tftypes.String, "username"),
				"password":             tftypes.NewValue(tftypes.String, ""),
				"insecure_skip_verify": tftypes.NewValue(tftypes.Bool, false),
				"timeout":              tftypes.NewValue(tftypes.Number, 30),
			},
			expectError:  true,
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

			actualError := response.Diagnostics.HasError()
			if actualError != tc.expectError {
				t.Errorf("Expected errors '%v', actual '%v'", tc.expectError, actualError)
			}
			found := false
			for _, err := range response.Diagnostics.Errors() {
				if strings.Contains(err.Summary(), tc.errorSummary) &&
					strings.Contains(err.Detail(), tc.errorDetail) {
					found = true
				}
			}
			if !found && tc.expectError {
				t.Errorf("Did not find error with expected summary '%v', detail containing '%v'. Actual errors %v",
					tc.errorSummary, tc.errorDetail, response.Diagnostics.Errors())
			}
		})
	}
}
