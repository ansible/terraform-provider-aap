package provider

import (
	"context"
	"errors"
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
	// This is a simple test and not table-driven. Unable to produce a failure in the
	// JSON marshaling because the struct is simple (strings only). Claude suggested
	// some invalid UTF-8 string sequences would trigger failure but they do not.

	model := EventStreamActionModel{
		Limit: types.StringValue("test-limit"),
	}
	buf, _ := model.CreateEventPayload()
	expectedItem := `"limit":"test-limit"`
	actual := string(buf)
	if !strings.Contains(actual, expectedItem) {
		t.Errorf("Expected to find item %q in payload, actual %q", expectedItem, actual)
	}
}

// Test CreateRequest
func TestCreateRequest(t *testing.T) {
	testTable := []struct {
		name          string
		context       context.Context
		username      string
		password      string
		url           string
		body          string
		expectAuth    string
		expectFailure bool
	}{
		{
			name:          "valid context produces POST request with auth header and body",
			context:       t.Context(),
			username:      "username",
			password:      "password",
			url:           "https://test.example.org",
			body:          "test-body",
			expectAuth:    "Basic dXNlcm5hbWU6cGFzc3dvcmQ=", // base64 encoding of string "username:password"
			expectFailure: false,
		},
		{
			name:          "empty context fails",
			context:       nil,
			expectFailure: true,
		},
	}

	for _, tc := range testTable {
		t.Run(tc.name, func(t *testing.T) {
			model := EventStreamActionModel{
				EventStreamConfig: EventStreamConfigModel{
					Username: types.StringValue(tc.username),
					Password: types.StringValue(tc.password),
					Url:      types.StringValue(tc.url),
				},
			}

			body := strings.NewReader(tc.body)
			req, diags := model.CreateRequest(tc.context, body)
			if tc.expectFailure {
				if diags.HasError() {
					// Failure expected, return
					return
				} else {
					t.Fatalf("Expecting success but received unexpected error %s", diags.Errors())
				}
			}

			// Check the method
			expectedMethod := http.MethodPost
			actualMethod := req.Method
			if actualMethod != expectedMethod {
				t.Errorf("Expected method %s, actual %s", expectedMethod, actualMethod)
			}

			actual := req.Header["Authorization"][0]
			if actual != tc.expectAuth {
				t.Errorf("Expected request to be created with auth header %q, actual %q", tc.expectAuth, actual)
			}
		})
	}
}

func TestCreateClient(t *testing.T) {
	t.Parallel()
	testTable := []struct {
		name                     string
		config                   EventStreamConfigModel
		expectInsecureSkipVerify bool
	}{
		{
			name:                     "CreateClient defaults to InsecureSkipVerify false",
			config:                   EventStreamConfigModel{},
			expectInsecureSkipVerify: false,
		},
		{
			name: "CreateClient honors InsecureSkipVerify true in config",
			config: EventStreamConfigModel{
				InsecureSkipVerify: types.BoolValue(true),
			},
			expectInsecureSkipVerify: true,
		},
		{
			name: "CreateClient honors InsecureSkipVerify false in config",
			config: EventStreamConfigModel{
				InsecureSkipVerify: types.BoolValue(false),
			},
			expectInsecureSkipVerify: false,
		},
		{
			name: "CreateClient defaults InsecureSkipVerify to false when unknown in config",
			config: EventStreamConfigModel{
				InsecureSkipVerify: types.BoolUnknown(),
			},
			expectInsecureSkipVerify: false,
		},
	}

	for _, tc := range testTable {
		t.Run(tc.name, func(t *testing.T) {
			model := EventStreamActionModel{
				EventStreamConfig: tc.config,
			}
			client := model.CreateClient()
			expected := tc.expectInsecureSkipVerify
			actual := client.Transport.(*http.Transport).TLSClientConfig.InsecureSkipVerify
			if actual != expected {
				t.Errorf("Expected client transport be created with InsecureSkipVerify %v, actual %v", expected, actual)
			}
		})
	}
}

type mockClient struct {
	StatusCode int
	Body       string
	Fail       bool
}

func (m *mockClient) Do(_ *http.Request) (*http.Response, error) {
	if m.Fail {
		return nil, errors.New("Test Error")
	} else {
		return &http.Response{
			StatusCode: m.StatusCode,
			Body:       io.NopCloser(strings.NewReader(m.Body)),
		}, nil
	}
}

type readFailer struct{}

func (f *readFailer) Read(_ []byte) (n int, err error) {
	return 0, errors.New("read failed")
}

func (f *readFailer) Close() error {
	return nil
}

type mockReadFailClient struct{}

func (m *mockReadFailClient) Do(_ *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       &readFailer{},
	}, nil
}

// Test ExecuteRequest
func TestExecuteRequest(t *testing.T) {
	t.Parallel()
	testTable := []struct {
		name          string
		client        HttpClient
		expectFailure bool
	}{
		{
			name:          "succeed when response status is http 200 ok",
			client:        &mockClient{StatusCode: http.StatusOK},
			expectFailure: false,
		},
		{
			name:          "succeed when response status is http 201 created",
			client:        &mockClient{StatusCode: http.StatusCreated},
			expectFailure: false,
		},
		{
			name:          "fail when response status is http 403 forbidden",
			client:        &mockClient{StatusCode: http.StatusForbidden},
			expectFailure: true,
		},
		{
			name:          "fail when client returns failure",
			client:        &mockClient{Fail: true},
			expectFailure: true,
		},
		{
			name:          "fail when reading the response fails",
			client:        &mockReadFailClient{},
			expectFailure: true,
		},
	}

	for _, tc := range testTable {
		t.Run(tc.name, func(t *testing.T) {
			a := EDAEventStreamAction{}
			req := http.Request{}
			_, diags := a.ExecuteRequest(tc.client, &req)
			if tc.expectFailure {
				if diags.HasError() {
					// Failure expected, return
					return
				} else {
					t.Fatalf("Expecting success but received unexpected error %s", diags.Errors())
				}
			}
		})
	}
}

// Acceptance tests use httptest to run a server, then applies config with actions
// and tests that that httptest server received the expected POST

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
