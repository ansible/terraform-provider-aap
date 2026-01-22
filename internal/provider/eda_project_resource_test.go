package provider

import (
	"encoding/json"
	"fmt"
	"net/url"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccEdaProject_basic(t *testing.T) {
	rName := "tf-test-" + acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)
	resourceName := "aap_eda_project.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			skipTestWithoutEDAPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckEdaProjectDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccEdaProjectConfig_basic(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckEdaProjectExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "name", rName),
					resource.TestCheckResourceAttrSet(resourceName, "organization_id"),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttrSet(resourceName, "url"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccEdaProject_disappears(t *testing.T) {
	rName := "tf-test-" + acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)
	resourceName := "aap_eda_project.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			skipTestWithoutEDAPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckEdaProjectDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccEdaProjectConfig_basic(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckEdaProjectExists(resourceName),
					testAccCheckEdaProjectDisappears(resourceName),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestAccEdaProject_description(t *testing.T) {
	rName := "tf-test-" + acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)
	resourceName := "aap_eda_project.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			skipTestWithoutEDAPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckEdaProjectDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccEdaProjectConfig_description(rName, "Initial description"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckEdaProjectExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "name", rName),
					resource.TestCheckResourceAttr(resourceName, "description", "Initial description"),
				),
			},
			{
				Config: testAccEdaProjectConfig_description(rName, "Updated description"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckEdaProjectExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "description", "Updated description"),
				),
			},
		},
	})
}

func TestAccEdaProject_scmBranch(t *testing.T) {
	rName := "tf-test-" + acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)
	resourceName := "aap_eda_project.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			skipTestWithoutEDAPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckEdaProjectDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccEdaProjectConfig_scmBranch(rName, "main"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckEdaProjectExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "scm_branch", "main"),
				),
			},
			{
				Config: testAccEdaProjectConfig_scmBranch(rName, "develop"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckEdaProjectExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "scm_branch", "develop"),
				),
			},
		},
	})
}

func testAccCheckEdaProjectDestroy(s *terraform.State) error {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "aap_eda_project" {
			continue
		}

		body, err := testMethodResource("GET", "/api/")
		if err != nil {
			return nil
		}

		var apiResponse AAPAPIEndpointResponse
		if jsonErr := json.Unmarshal(body, &apiResponse); jsonErr != nil {
			return fmt.Errorf("error parsing API response: %w", jsonErr)
		}

		if apiResponse.APIs.EDA == "" {
			return nil
		}

		edaVersionBody, err := testMethodResource("GET", apiResponse.APIs.EDA)
		if err != nil {
			return nil
		}

		var edaResponse AAPAPIEndpointResponse
		if jsonErr := json.Unmarshal(edaVersionBody, &edaResponse); jsonErr != nil {
			return fmt.Errorf("error parsing EDA response: %w", jsonErr)
		}

		var edaPath string
		if len(edaResponse.CurrentVersion) > 0 {
			if parsed, parseErr := url.Parse(edaResponse.CurrentVersion); parseErr == nil {
				edaPath = parsed.Path
			}
		}

		if edaPath == "" {
			return nil
		}

		projectsURL := fmt.Sprintf("%s/projects", edaPath)
		params := map[string]string{
			"name": rs.Primary.Attributes["name"],
		}

		listBody, err := testMethodResourceWithParams("GET", projectsURL, params)
		if err != nil {
			return nil
		}

		var listResponse EdaProjectListResponse
		if jsonErr := json.Unmarshal(listBody, &listResponse); jsonErr != nil {
			return fmt.Errorf("error parsing response: %w", jsonErr)
		}

		if listResponse.Count > 0 {
			return fmt.Errorf("EDA Project %s still exists", rs.Primary.Attributes["name"])
		}
	}

	return nil
}

func testAccCheckEdaProjectExists(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", resourceName)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("No EDA Project ID is set")
		}

		body, err := testMethodResource("GET", "/api/")
		if err != nil {
			return fmt.Errorf("error fetching API info: %w", err)
		}

		var apiResponse AAPAPIEndpointResponse
		if jsonErr := json.Unmarshal(body, &apiResponse); jsonErr != nil {
			return fmt.Errorf("error parsing API response: %w", jsonErr)
		}

		if apiResponse.APIs.EDA == "" {
			return fmt.Errorf("EDA API not available")
		}

		edaVersionBody, err := testMethodResource("GET", apiResponse.APIs.EDA)
		if err != nil {
			return fmt.Errorf("error fetching EDA version: %w", err)
		}

		var edaResponse AAPAPIEndpointResponse
		if jsonErr := json.Unmarshal(edaVersionBody, &edaResponse); jsonErr != nil {
			return fmt.Errorf("error parsing EDA response: %w", jsonErr)
		}

		var edaPath string
		if len(edaResponse.CurrentVersion) > 0 {
			if parsed, parseErr := url.Parse(edaResponse.CurrentVersion); parseErr == nil {
				edaPath = parsed.Path
			}
		}

		if edaPath == "" {
			return fmt.Errorf("could not determine EDA path")
		}

		projectsURL := fmt.Sprintf("%s/projects", edaPath)
		params := map[string]string{
			"name": rs.Primary.Attributes["name"],
		}

		listBody, err := testMethodResourceWithParams("GET", projectsURL, params)
		if err != nil {
			return fmt.Errorf("error fetching EDA project: %w", err)
		}

		var listResponse EdaProjectListResponse
		if jsonErr := json.Unmarshal(listBody, &listResponse); jsonErr != nil {
			return fmt.Errorf("error parsing response: %w", jsonErr)
		}

		if listResponse.Count == 0 {
			return fmt.Errorf("EDA Project not found")
		}

		return nil
	}
}

func testAccCheckEdaProjectDisappears(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", resourceName)
		}

		// Discover EDA endpoint
		body, err := testMethodResource("GET", "/api/")
		if err != nil {
			return fmt.Errorf("error fetching API info: %w", err)
		}

		var apiResponse AAPAPIEndpointResponse
		if jsonErr := json.Unmarshal(body, &apiResponse); jsonErr != nil {
			return fmt.Errorf("error parsing API response: %w", jsonErr)
		}

		if apiResponse.APIs.EDA == "" {
			return fmt.Errorf("EDA API not available")
		}

		edaVersionBody, err := testMethodResource("GET", apiResponse.APIs.EDA)
		if err != nil {
			return fmt.Errorf("error fetching EDA version: %w", err)
		}

		var edaResponse AAPAPIEndpointResponse
		if jsonErr := json.Unmarshal(edaVersionBody, &edaResponse); jsonErr != nil {
			return fmt.Errorf("error parsing EDA response: %w", jsonErr)
		}

		var edaPath string
		if len(edaResponse.CurrentVersion) > 0 {
			if parsed, parseErr := url.Parse(edaResponse.CurrentVersion); parseErr == nil {
				edaPath = parsed.Path
			}
		}

		if edaPath == "" {
			return fmt.Errorf("could not determine EDA path")
		}

		projectURL := fmt.Sprintf("%s/projects/%s", edaPath, rs.Primary.ID)
		_, err = testDeleteResource(projectURL)
		if err != nil {
			return fmt.Errorf("error deleting EDA project: %w", err)
		}

		return nil
	}
}

func testAccEdaProjectConfig_basic(rName string) string {
	return testAccEdaProjectConfig_organization(rName, "Default")
}

func testAccEdaProjectConfig_organization(rName, orgName string) string {
	return fmt.Sprintf(`
data "aap_organization" "test" {
  name = %[2]q
}

resource "aap_eda_project" "test" {
  name            = %[1]q
  url             = "https://github.com/ansible/terraform-provider-aap-test.git"
  organization_id = data.aap_organization.test.id
}
`, rName, orgName)
}

func testAccEdaProjectConfig_description(rName, description string) string {
	return fmt.Sprintf(`
data "aap_organization" "test" {
  name = "Default"
}

resource "aap_eda_project" "test" {
  name            = %[1]q
  description     = %[2]q
  url             = "https://github.com/ansible/terraform-provider-aap-test.git"
  organization_id = data.aap_organization.test.id
}
`, rName, description)
}

func testAccEdaProjectConfig_scmBranch(rName, branch string) string {
	return fmt.Sprintf(`
data "aap_organization" "test" {
  name = "Default"
}

resource "aap_eda_project" "test" {
  name            = %[1]q
  url             = "https://github.com/ansible/terraform-provider-aap-test.git"
  scm_branch      = %[2]q
  organization_id = data.aap_organization.test.id
}
`, rName, branch)
}
