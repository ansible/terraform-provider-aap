package provider

import (
	"encoding/json"
	"fmt"
	"regexp"
	"testing"

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
		PreCheck: func() {
			testAccPreCheck(t)
			skipTestWithoutEDAPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccEventStream(eventStreamName),
				Check:  resource.TestMatchResourceAttr("data.aap_eda_eventstream.test", "url", reEventStreamPostURL),
			},
		},
	})
}

func skipTestWithoutEDAPreCheck(t testing.TB) {
	t.Helper()

	body, err := testMethodResource("GET", "/api/")
	if err != nil {
		t.Errorf("error fetching /api/ endpoint: %v", err)
	}

	var response AAPApiEndpointResponse

	err = json.Unmarshal(body, &response)
	if err != nil {
		t.Errorf("error unmarshalling response body during AAP version pre-check: %v", err)
	}

	if response.Apis.EDA == "" {
		t.Skip("EDA API endpoint not found: skipping test")
	}
}

func testAccEventStream(name string) string {
	return fmt.Sprintf(`
	data "aap_eda_eventstream" "test" {
		name = "%s"
	}
	`, name)
}
