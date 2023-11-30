package provider

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

func TestJobResourceModelParseHttpResponse(t *testing.T) {
	template_id := basetypes.NewInt64Value(1)
	inventory_id := basetypes.NewInt64Value(2)
	extra_vars := basetypes.NewStringNull()
	testTable := []struct {
		name     string
		body     []byte
		expected jobResourceModel
		failure  bool
	}{
		{
			name:    "no ignored fields",
			failure: false,
			body:    []byte(`{"job_type": "run", "url": "/api/v2/jobs/14/", "status": "pending"}`),
			expected: jobResourceModel{
				TemplateId:    template_id,
				Type:          types.StringValue("run"),
				URL:           types.StringValue("/api/v2/jobs/14/"),
				Status:        types.StringValue("pending"),
				InventoryId:   inventory_id,
				ExtraVars:     extra_vars,
				IgnoredFields: types.ListNull(types.StringType),
			},
		},
		{
			name:    "ignored fields",
			failure: false,
			body:    []byte(`{"job_type": "run", "url": "/api/v2/jobs/14/", "status": "pending", "ignored_fields": {"extra_vars": "{\"bucket_state\":\"absent\"}"}}`),
			expected: jobResourceModel{
				TemplateId:    template_id,
				Type:          types.StringValue("run"),
				URL:           types.StringValue("/api/v2/jobs/14/"),
				Status:        types.StringValue("pending"),
				InventoryId:   inventory_id,
				ExtraVars:     extra_vars,
				IgnoredFields: basetypes.NewListValueMust(types.StringType, []attr.Value{types.StringValue("extra_vars")}),
			},
		},
		{
			name:     "bad json",
			failure:  true,
			body:     []byte(`{job_type: run}`),
			expected: jobResourceModel{},
		},
	}
	for _, tc := range testTable {
		t.Run(tc.name, func(t *testing.T) {
			d := jobResourceModel{
				TemplateId:  template_id,
				InventoryId: inventory_id,
				ExtraVars:   extra_vars,
			}
			err := d.ParseHttpResponse(tc.body)
			if tc.failure {
				if err == nil {
					t.Errorf("expecting failure while the process has not failed")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected process failure (%s)", err.Error())
				} else if !reflect.DeepEqual(tc.expected, d) {
					t.Errorf("expected (%v) - result (%v)", tc.expected, d)
				}
			}
		})
	}
}

func toString(b *bytes.Reader) string {
	if b == nil {
		return ""
	}
	buf := new(strings.Builder)
	io.Copy(buf, b)
	return buf.String()
}

func toJson(b *bytes.Reader) map[string]interface{} {
	var result map[string]interface{}
	err := json.Unmarshal([]byte(toString(b)), &result)
	if err != nil {
		return make(map[string]interface{})
	}
	return result
}

func TestJobResourceModelCreateRequestBody(t *testing.T) {
	testTable := []struct {
		name     string
		input    jobResourceModel
		expected *bytes.Reader
	}{
		{
			name: "unknown fields",
			input: jobResourceModel{
				ExtraVars:   basetypes.NewStringUnknown(),
				InventoryId: basetypes.NewInt64Unknown(),
			},
			expected: nil,
		},
		{
			name: "null fields",
			input: jobResourceModel{
				ExtraVars:   basetypes.NewStringNull(),
				InventoryId: basetypes.NewInt64Null(),
			},
			expected: nil,
		},
		{
			name: "extra vars only",
			input: jobResourceModel{
				ExtraVars:   types.StringValue("{\"test_name\":\"extra_vars\", \"provider\":\"aap\"}"),
				InventoryId: basetypes.NewInt64Null(),
			},
			expected: bytes.NewReader([]byte(`{"extra_vars":{"test_name":"extra_vars","provider":"aap"}}`)),
		},
		{
			name: "inventory vars only",
			input: jobResourceModel{
				ExtraVars:   basetypes.NewStringNull(),
				InventoryId: basetypes.NewInt64Value(201),
			},
			expected: bytes.NewReader([]byte(`{"inventory": 201}`)),
		},
		{
			name: "combined",
			input: jobResourceModel{
				ExtraVars:   types.StringValue("{\"test_name\":\"extra_vars\", \"provider\":\"aap\"}"),
				InventoryId: basetypes.NewInt64Value(3),
			},
			expected: bytes.NewReader([]byte(`{"inventory": 3, "extra_vars":{"test_name":"extra_vars","provider":"aap"}}`)),
		},
	}

	for _, tc := range testTable {
		t.Run(tc.name, func(t *testing.T) {
			data, _ := tc.input.CreateRequestBody()
			if !reflect.DeepEqual(toJson(tc.expected), toJson(data)) {
				t.Errorf("expected (%s)", toString(tc.expected))
				t.Errorf("computed (%s)", toString(data))
			}
		})
	}
}

type MockJobResource struct {
	Id        string
	URL       string
	Inventory string
	Response  map[string]string
}

func NewMockJobResource(Id, Inventory, Url string) *MockJobResource {
	return &MockJobResource{
		Id:        Id,
		URL:       Url,
		Inventory: Inventory,
		Response:  map[string]string{},
	}
}

func (d *MockJobResource) GetTemplateId() string {
	return d.Id
}

func (d *MockJobResource) GetURL() string {
	return d.URL
}

func (d *MockJobResource) ParseHttpResponse(body []byte) error {
	err := json.Unmarshal(body, &d.Response)
	if err != nil {
		return err
	}
	return nil
}

func (d *MockJobResource) CreateRequestBody() (*bytes.Reader, error) {
	if len(d.Inventory) == 0 {
		return nil, nil
	}
	m := map[string]string{"Inventory": d.Inventory}
	json_raw, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(json_raw), nil
}

type MockHttpClient struct {
	accept_methods []string
	http_code      int
}

func NewMockHttpClient(methods []string, http_code int) *MockHttpClient {
	return &MockHttpClient{
		accept_methods: methods,
		http_code:      http_code,
	}
}

func mergeStringMaps(m1 map[string]string, m2 map[string]string) map[string]string {
	merged := make(map[string]string)
	for k, v := range m1 {
		merged[k] = v
	}
	for k, v := range m2 {
		merged[k] = v
	}
	return merged
}

var mResponse1 = map[string]string{
	"status": "running",
	"type":   "check",
}

var mResponse2 = map[string]string{
	"status":   "pending",
	"playbook": "ansible_aws.yaml",
}

var mResponse3 = map[string]string{
	"status":                "complete",
	"execution_environment": "3",
}

func (c *MockHttpClient) doRequest(method string, path string, data io.Reader) (int, []byte, error) {

	config := map[string]map[string]string{
		"/api/v2/job_templates/1/launch/": mResponse1,
		"/api/v2/job_templates/2/launch/": mResponse2,
		"/api/v2/jobs/1/":                 mResponse1,
		"/api/v2/jobs/2/":                 mResponse3,
	}

	if !slices.Contains(c.accept_methods, method) {
		return http.StatusBadRequest, nil, nil
	}
	response, ok := config[path]
	if !ok {
		return http.StatusNotFound, nil, nil
	}

	if data != nil {
		// add request info into response
		buf := new(strings.Builder)
		io.Copy(buf, data)
		var m_data map[string]string
		err := json.Unmarshal([]byte(buf.String()), &m_data)
		if err != nil {
			return -1, nil, err
		}
		response = mergeStringMaps(response, m_data)
	}
	result, err := json.Marshal(response)
	if err != nil {
		return -1, nil, err
	}
	return c.http_code, result, nil
}

func TestJobResourceCreateJob(t *testing.T) {
	testTable := []struct {
		name           string
		Id             string
		Inventory      string
		expected       map[string]string
		accept_methods []string
		http_code      int
		failed         bool
	}{
		{
			name:           "create job simple job (no request data)",
			Id:             "1",
			Inventory:      "",
			http_code:      http.StatusCreated,
			failed:         false,
			accept_methods: []string{"POST", "post"},
			expected:       mResponse1,
		},
		{
			name:           "create job with request data",
			Id:             "1",
			Inventory:      "3",
			http_code:      http.StatusCreated,
			failed:         false,
			accept_methods: []string{"POST", "post"},
			expected:       mergeStringMaps(mResponse1, map[string]string{"Inventory": "3"}),
		},
		{
			name:           "try with non existing template id",
			Id:             "-1",
			Inventory:      "3",
			http_code:      http.StatusCreated,
			failed:         true,
			accept_methods: []string{"POST", "post"},
			expected:       nil,
		},
		{
			name:           "Unexpected method leading to not found",
			Id:             "1",
			Inventory:      "3",
			http_code:      http.StatusCreated,
			failed:         true,
			accept_methods: []string{"GET", "get"},
			expected:       nil,
		},
		{
			name:           "using another template id",
			Id:             "2",
			Inventory:      "1",
			http_code:      http.StatusCreated,
			failed:         false,
			accept_methods: []string{"POST", "post"},
			expected:       mergeStringMaps(mResponse2, map[string]string{"Inventory": "1"}),
		},
	}

	for _, tc := range testTable {
		t.Run(tc.name, func(t *testing.T) {
			resource := NewMockJobResource(tc.Id, tc.Inventory, "")

			job := JobResource{
				client: NewMockHttpClient(tc.accept_methods, tc.http_code),
			}
			err := job.CreateJob(resource)
			if (tc.failed && err == nil) || (!tc.failed && err != nil) {
				if err != nil {
					t.Errorf("process has failed with (%s) while it should not", err.Error())
				} else {
					t.Errorf("failure expected but the process did not failed!!")
				}
			} else if !tc.failed && !reflect.DeepEqual(tc.expected, resource.Response) {
				t.Errorf("expected (%v)", tc.expected)
				t.Errorf("computed (%v)", resource.Response)
			}
		})
	}
}

func TestJobResourceReadJob(t *testing.T) {
	testTable := []struct {
		name           string
		url            string
		expected       map[string]string
		accept_methods []string
		http_code      int
		failed         bool
	}{
		{
			name:           "Read existing job",
			url:            "/api/v2/jobs/1/",
			http_code:      http.StatusOK,
			failed:         false,
			accept_methods: []string{"GET", "get"},
			expected:       mResponse1,
		},
		{
			name:           "Read another job",
			url:            "/api/v2/jobs/2/",
			http_code:      http.StatusOK,
			failed:         false,
			accept_methods: []string{"GET", "get"},
			expected:       mResponse3,
		},
		{
			name:           "GET not part of accepted methods",
			url:            "/api/v2/jobs/2/",
			http_code:      http.StatusOK,
			failed:         true,
			accept_methods: []string{"HEAD"},
			expected:       nil,
		},
		{
			name:           "no url provided",
			url:            "",
			http_code:      http.StatusOK,
			failed:         false,
			accept_methods: []string{"GET", "get"},
			expected:       map[string]string{},
		},
	}

	for _, tc := range testTable {
		t.Run(tc.name, func(t *testing.T) {
			resource := NewMockJobResource("", "", tc.url)

			job := JobResource{
				client: NewMockHttpClient(tc.accept_methods, tc.http_code),
			}
			err := job.ReadJob(resource)
			if (tc.failed && err == nil) || (!tc.failed && err != nil) {
				if err != nil {
					t.Errorf("process has failed with (%s) while it should not", err.Error())
				} else {
					t.Errorf("failure expected but the process did not failed!!")
				}
			} else if !tc.failed && !reflect.DeepEqual(tc.expected, resource.Response) {
				t.Errorf("expected (%v)", tc.expected)
				t.Errorf("computed (%v)", resource.Response)
			}
		})
	}
}
