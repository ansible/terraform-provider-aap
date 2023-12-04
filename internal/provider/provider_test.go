package provider

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"testing"

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

func testGetResource(urlPath string) (map[string]interface{}, error) {

	host := os.Getenv("AAP_HOST")
	username := os.Getenv("AAP_USERNAME")
	password := os.Getenv("AAP_PASSWORD")

	insecure_skip_verify, _ := strconv.ParseBool(os.Getenv("AAP_INSECURE_SKIP_VERIFY"))
	resource_url, _ := url.JoinPath(host, urlPath)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: insecure_skip_verify},
	}
	client := http.Client{Transport: tr}

	req, _ := http.NewRequest("GET", resource_url, nil)
	req.SetBasicAuth(username, password)

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get request on URL='%s' returned http code [%d]", resource_url, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	return result, err
}
