package provider

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/provider"
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

func testGetResource(urlPath string) ([]byte, error) {
	host := os.Getenv("AAP_HOST")
	username := os.Getenv("AAP_USERNAME")
	password := os.Getenv("AAP_PASSWORD")

	resourceURL, _ := url.JoinPath(host, urlPath)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := http.Client{Transport: tr}

	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, resourceURL, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(username, password)

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get request on URL='%s' returned http code [%d]", resourceURL, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return body, err
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
