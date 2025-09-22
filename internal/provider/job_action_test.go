package provider

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccAAPJobAction_basic(t *testing.T) {
	jobTemplateID := os.Getenv("AAP_TEST_JOB_TEMPLATE_ID")
	randNum, _ := rand.Int(rand.Reader, big.NewInt(50000000))
	inventoryName := fmt.Sprintf("%s-%d", "tf-acc", randNum.Int64())
	timestamp := time.Now().Format("20060102150405")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccJobResourcePreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccBasicJobAction(inventoryName, jobTemplateID),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccJobLaunched(t, timestamp),
				),
			},
		},
	})
}

func getAllJobResults(results []interface{}, urlPath string, params map[string]string) ([]interface{}, error) {
	body, err := testMethodResourceWithParams(http.MethodGet, urlPath, params)
	if err != nil {
		return nil, err
	}

	var data map[string]any
	err = json.Unmarshal(body, &data)
	if err != nil {
		return nil, err
	}
	r, ok := data["results"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("results field not found or not an array")
	}
	results = append(results, r...)

	if data["next"] != nil {
		next, ok := data["next"].(string)
		if !ok {
			return nil, fmt.Errorf("next field not found or not a string")
		}
		// Parse the next URL to extract path and query parameters
		nextURL, err := url.Parse(next)
		if err != nil {
			return nil, fmt.Errorf("failed to parse next URL: %v", err)
		}

		// Convert query parameters to map
		nextParams := make(map[string]string)
		for key, values := range nextURL.Query() {
			if len(values) > 0 {
				nextParams[key] = values[0]
			}
		}

		return getAllJobResults(results, nextURL.Path, nextParams)
	}
	return results, nil
}

func testAccJobLaunched(_ *testing.T, timestamp string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		params := map[string]string{
			"unified_job_template": os.Getenv("AAP_TEST_JOB_TEMPLATE_ID"),
		}
		results, err := getAllJobResults(nil, "api/controller/v2/jobs", params)
		if err != nil {
			return err
		}

		targetTime, err := time.Parse("20060102150405", timestamp)
		if err != nil {
			return fmt.Errorf("failed to parse timestamp: %v", err)
		}

		for _, result := range results {
			resultMap, ok := result.(map[string]interface{})
			if !ok {
				continue
			}
			createdStr, ok := resultMap["created"].(string)
			if !ok {
				continue
			}

			jobTemplateID, ok := resultMap["summary_fields"].(map[string]interface{})["job_template"].(map[string]interface{})["id"].(float64)
			if !ok {
				continue
			}
			if fmt.Sprintf("%v", jobTemplateID) != os.Getenv("AAP_TEST_JOB_TEMPLATE_ID") {
				continue
			}

			// Parse the job creation time (assuming RFC3339 format from API)
			jobTime, err := time.Parse(time.RFC3339, createdStr)
			if err != nil {
				continue
			}

			// Check if job was created within 5 minutes of the target timestamp
			timeDiff := jobTime.Sub(targetTime).Abs()
			if timeDiff <= 5*time.Minute {
				return nil // Found a job within the time window
			}
		}

		return fmt.Errorf("no job found within 5 minutes of timestamp %s", timestamp)
	}
}

func testAccBasicJobAction(inventoryName, jobTemplateID string) string {
	return fmt.Sprintf(`
resource "aap_inventory" "test" {
	name = "%s"
	lifecycle {
		action_trigger {
			events = [after_create]
			actions = [action.aap_job.test]
		}
	}
}

action "aap_job" "test" {
	config {
		job_template_id 	= %s
		wait_for_completion = true
	}
}
`, inventoryName, jobTemplateID)
}
