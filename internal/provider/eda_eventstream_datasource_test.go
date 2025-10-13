package provider

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var reEventStreamPostURL = regexp.MustCompile(`/api/eda/v1/external_event_stream/[a-zA-Z0-9_-]+/post`)

func TestNewEDAEventStreamDataSource(t *testing.T) {
	testDataSource := NewEDAEventStreamDataSource()

	expectedMetadataEntitySlug := "eda_eventstream"
	expectedDescriptiveEntityName := "EDA Event Stream"
	expectedApiEntitySlug := "event-streams"

	switch v := testDataSource.(type) {
	case *EDAEventStreamDataSource:
		if v.ApiEntitySlug != expectedApiEntitySlug {
			t.Errorf("Incorrect ApiEntitySlug. Got: %s, wanted: %s", v.ApiEntitySlug, expectedApiEntitySlug)
		}
		if v.DescriptiveEntityName != expectedDescriptiveEntityName {
			t.Errorf("Incorrect DescriptiveEntityName. Got: %s, wanted: %s", v.DescriptiveEntityName, expectedDescriptiveEntityName)
		}
		if v.MetadataEntitySlug != expectedMetadataEntitySlug {
			t.Errorf("Incorrect MetadataEntitySlug. Got: %s, wanted: %s", v.MetadataEntitySlug, expectedMetadataEntitySlug)
		}
	default:
		t.Errorf("Incorrect datasource type returned. Got: %T, wanted: %T", v, testDataSource)
	}
}

// TestAccEDAEventStreamDataSourceRetrievesPostURL ensures the aap_eda_eventstream datasource can retrieve
// an EDA Event Stream post URL successfully.
func TestAccEDAEventStreamDataSourceRetrievesPostURL(t *testing.T) {
	eventStreamName := "Test Event Stream"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { aapVersionPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccEventStream(eventStreamName),
				Check:  resource.TestMatchResourceAttr("data.aap_eda_eventstream.test", "url", reEventStreamPostURL),
			},
		},
	})
}

// aapVersionPreCheck determines the AAP version before an acceptance test is executed. The test is skipped
// if the version is prior to AAP 2.5.
func aapVersionPreCheck(t testing.TB) {
	t.Helper()

	timeoutSec := int64(3)
	httpTransport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	httpClient := &http.Client{Transport: httpTransport, Timeout: time.Duration(timeoutSec) * time.Second}

	hostname := os.Getenv("AAP_HOSTNAME")
	if hostname == "" {
		t.Error("AAP_HOSTNAME environment variable was empty during AAP version pre-check")
	}

	url, err := url.JoinPath(hostname, "/api/")
	if err != nil {
		t.Errorf("error constructing URL during AAP version pre-check: %v", err)
	}

	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		t.Errorf("error creating request during AAP version pre-check: %v", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		t.Errorf("error in response during AAP version pre-check: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Errorf("error reading response body during AAP version pre-check: %v", err)
	}

	var apiEndpointResp struct {
		Description string `json:"description"`
	}
	err = json.Unmarshal(body, &apiEndpointResp)
	if err != nil {
		t.Errorf("error unmarshalling response body during AAP version pre-check: %v", err)
	}

	if apiEndpointResp.Description == "" {
		t.Error("empty API description during AAP version check")
	}

	// AAP 2.4's description does not mention "gateway"
	// AAP 2.4 -> AWX REST API
	// AAP 2.5 -> AAP gateway REST API
	if !strings.Contains(apiEndpointResp.Description, "gateway") {
		t.Skip("EDA is not supported prior to AAP 2.5")
	}
}

func testAccEventStream(name string) string {
	return fmt.Sprintf(`
	data "aap_eda_eventstream" "test" {
		name = "%s"
	}
	`, name)
}
