package provider

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	fwaction "github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

func TestEDAEventStreamActionSchema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schemaRequest := fwaction.SchemaRequest{}
	schemaResponse := fwaction.SchemaResponse{}

	NewEDAEventStreamAction().Schema(ctx, schemaRequest, &schemaResponse)

	if schemaResponse.Diagnostics.HasError() {
		t.Fatalf("Schema method diagnostics: %+v", schemaResponse.Diagnostics)
	}

	// Validate the schema
	diagnostics := schemaResponse.Schema.ValidateImplementation(ctx)

	if diagnostics.HasError() {
		t.Fatalf("Schema validation diagnostics: %+v", diagnostics)
	}
}

// Test Metadata
func TestEDAEventStreamActionMetadata(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	metadataRequest := fwaction.MetadataRequest{
		ProviderTypeName: "test",
	}
	metadataResponse := fwaction.MetadataResponse{}

	NewEDAEventStreamAction().Metadata(ctx, metadataRequest, &metadataResponse)
	expected := "test_eda_eventstream"
	actual := metadataResponse.TypeName
	if expected != actual {
		t.Errorf("Expected metadata TypeName %q, received %q", expected, actual)
	}
}

// Test CreateEventPayload
func TestCreateEventPayload(t *testing.T) {
	t.Parallel()
	// Creates a JSON
	// write a test table here
	model := EventStreamActionModel{
		Limit: types.StringValue("test-limit"),
	}
	buf, _ := model.CreateEventPayload()
	expectedItem := `"limit":"test-limit"`
	actual := string(buf)
	if !strings.Contains(actual, expectedItem) {
		t.Errorf("Expected to find item %q in payload, actual %q", expectedItem, actual)
	}
	// TODO: Test error case if possible
}

// Test CreateRequest
func TestCreateRequest(t *testing.T) {
	t.Parallel()
	model := EventStreamActionModel{
		EventStreamConfig: EventStreamConfigModel{
			Username: types.StringValue("username"),
			Password: types.StringValue("password"),
			Url:      types.StringValue("https://test.example.org"),
		},
	}

	body := strings.NewReader("test-body")
	req, _ := model.CreateRequest(context.TODO(), body)
	actual := req.Header["Authorization"][0]
	// base64 encoding of string "username:password"
	expected := "Basic dXNlcm5hbWU6cGFzc3dvcmQ="
	if actual != expected {
		t.Errorf("Expected request to be created with basic auth header %q, actual %q", expected, actual)
	}
	// TODO: Test that it's a POST and the body
	// TODO: Test the failure cases, maybe with a canceled context
}

func TestCreateClient(t *testing.T) {
	t.Parallel()
	model := EventStreamActionModel{
		EventStreamConfig: EventStreamConfigModel{
			InsecureSkipVerify: types.BoolValue(true),
		},
	}
	client := model.CreateClient()
	expected := true
	actual := client.Transport.(*http.Transport).TLSClientConfig.InsecureSkipVerify
	if actual != expected {
		t.Errorf("Expected client transport be created with InsecureSkipVerify %v, actual %v", expected, actual)
	}
	// TODO: Check other value of insecure
}

type MockClient struct {
	StatusCode int
	Body       string
}

func (m *MockClient) Do(_ *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: m.StatusCode,
		Body:       io.NopCloser(strings.NewReader(m.Body)),
	}, nil
}

// Test ExecuteRequest
func TestExecuteRequest(t *testing.T) {
	t.Parallel()
	a := EDAEventStreamAction{}

	client := MockClient{StatusCode: http.StatusOK, Body: "test"}
	body, diags := a.ExecuteRequest(&client, nil)
	if diags.HasError() {
		t.Errorf("Unexpected error in ExecuteRequest: %s", diags.Errors())
	}

	expected := "test"
	actual := string(body)
	if actual != expected {
		t.Errorf("Expected ExecuteRequest to return body %v, actual %v", expected, actual)
	}
	// TODO: Test client.Do error
	// TODO: Test http error codes
}

// Acceptance testing will use httptest to run a server and test that actions POST to it

type testHandler struct {
	callCount     int
	requestMethod string
	responseCode  int
	requestBody   string
	requestBytes  int
	requestError  error
	responseBody  string
	responseBytes int
	responseError error
}

func (h *testHandler) ServeHTTP(writer http.ResponseWriter, req *http.Request) {
	h.callCount += 1

	// Read the request and record the length
	buffer := make([]byte, req.ContentLength)
	h.requestBytes, h.requestError = req.Body.Read(buffer)
	h.requestBody = string(buffer)
	h.requestMethod = req.Method

	// write the response
	writer.WriteHeader(h.responseCode)
	h.responseBytes, h.responseError = writer.Write([]byte(h.responseBody))
}

// Test Invoke (this should be an acceptance test)
func TestAccEDAEventStreamAction(t *testing.T) {
	// Create an http test server
	handler := testHandler{
		responseCode: http.StatusOK,
	}
	testServer := httptest.NewServer(&handler)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() {},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_14_0),
		},
		Steps: []resource.TestStep{
			{
				Config: testAccBasicAction("after_create", testServer.URL),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("terraform_data.trigger", "input", "test"),
					func(_ *terraform.State) error {
						testServer.Close()
						return testAccCheckActionReceived(t, &handler)
					},
				),
			},
		},
	})
	// TODO: Test that no actions are fired the second time
	// TODO: Test that no actions are fired if lifecycle doesn't match
}

func testAccCheckActionReceived(t *testing.T, handler *testHandler) error {
	t.Helper()
	// should be a POST
	if handler.requestMethod != http.MethodPost {
		return fmt.Errorf("Expected method %v, received %v", http.MethodPost, handler.requestMethod)
	}
	// Action should be received
	expected := 132 // Length of the sample JSON
	actual := handler.requestBytes
	if expected != actual {
		return fmt.Errorf("Expected %v bytes, received %v. Request body %s", expected, actual, handler.requestBody)
	}
	if handler.callCount != 1 {
		return fmt.Errorf("Expected 1 call, received %v.", handler.callCount)
	}

	expectedBody := `{"limit":"limit","template_type":"job","job_template_name":"template",` +
		`"workflow_job_template_name":"","organization_name":"Default"}`
	actualBody := handler.requestBody
	if actualBody != expectedBody {
		return fmt.Errorf("Unexpected request body %s", actualBody)
	}
	return nil
}

func testAccBasicAction(trigger_events string, url string) string {
	return fmt.Sprintf(`
	resource "terraform_data" "trigger" {
		input = "test"
		lifecycle {
			action_trigger {
				events = [%s]
				actions = [action.aap_eda_eventstream.action]
			}
		}
	}

	action "aap_eda_eventstream" "action" {
		config {
			limit = "limit"
			template_type = "job"
			job_template_name = "template"
			organization_name = "Default"
			event_stream_config = {
				username = "username"
				password = "password"
				url = "%s"
			}
		}

	}
	`, trigger_events, url)
}
