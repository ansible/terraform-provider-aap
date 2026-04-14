package provider

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ action.Action = (*EDAEventStreamPostAction)(nil)
)

func NewEDAEventStreamPostAction() action.Action {
	return &EDAEventStreamPostAction{}
}

type EDAEventStreamPostAction struct{}

// Metadata
func (a *EDAEventStreamPostAction) Metadata(_ context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_eda_eventstream_post"
}

// Schema
func (a *EDAEventStreamPostAction) Schema(_ context.Context, _ action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Sends an event payload to an EDA Event Stream URL",
		Attributes: map[string]schema.Attribute{
			"limit": schema.StringAttribute{
				Description: "Ansible limit for job execution",
				Required:    true,
			},
			"template_type": schema.StringAttribute{
				Description: "Template type: either job or workflow_job",
				Required:    true,
			},
			"job_template_name": schema.StringAttribute{
				Description: "Job Template Name (Used when template_type is job)",
				Optional:    true,
			},
			"workflow_job_template_name": schema.StringAttribute{
				Description: "Workflow Job Template Name (Used when template_type is workflow_job)",
				Optional:    true,
			},
			"organization_name": schema.StringAttribute{
				Description: "Organization Name",
				Required:    true,
			},
			"event_stream_config": schema.SingleNestedAttribute{
				Description: "Details for the EDA Event Stream",
				Required:    true,
				Attributes: map[string]schema.Attribute{
					"url": schema.StringAttribute{
						Description: "URL to receive the event POST",
						Required:    true,
					},
					"insecure_skip_verify": schema.BoolAttribute{
						Description: "Disable TLS verification (insecure)",
						Optional:    true,
					},
					"username": schema.StringAttribute{
						Description: "Username to use when performing the POST to the Event Stream URL",
						Required:    true,
					},
					"password": schema.StringAttribute{
						Description: "Password to use when performing the POST to the Event Stream URL",
						Required:    true,
					},
				},
			},
		},
	}
}

type EventStreamConfigModel struct {
	Url                types.String `tfsdk:"url"`
	Username           types.String `tfsdk:"username"`
	Password           types.String `tfsdk:"password"`
	InsecureSkipVerify types.Bool   `tfsdk:"insecure_skip_verify"`
}

type EventStreamActionModel struct {
	Limit                   types.String           `tfsdk:"limit"`
	TemplateType            types.String           `tfsdk:"template_type"`
	JobTemplateName         types.String           `tfsdk:"job_template_name"`
	WorkflowJobTemplateName types.String           `tfsdk:"workflow_job_template_name"`
	OrganizationName        types.String           `tfsdk:"organization_name"`
	EventStreamConfig       EventStreamConfigModel `tfsdk:"event_stream_config"`
}

type EventStreamConfigAPIModel struct {
	Limit                   string `json:"limit"`
	TemplateType            string `json:"template_type"`
	JobTemplateName         string `json:"job_template_name"`
	WorkflowJobTemplateName string `json:"workflow_job_template_name"`
	OrganizationName        string `json:"organization_name"`
}

type JSONMarshaler interface {
	Marshal(v any) ([]byte, error)
}

type defaultJSONMarshaler struct{}

func (d defaultJSONMarshaler) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func (m *EventStreamActionModel) CreateEventPayload() ([]byte, diag.Diagnostics) {
	return m.CreateEventPayloadWithMarshaler(defaultJSONMarshaler{})
}

func (m *EventStreamActionModel) CreateEventPayloadWithMarshaler(marshaler JSONMarshaler) ([]byte, diag.Diagnostics) {
	// Convert to the API Model
	payload := EventStreamConfigAPIModel{
		TemplateType:            m.TemplateType.ValueString(),
		JobTemplateName:         m.JobTemplateName.ValueString(),
		WorkflowJobTemplateName: m.WorkflowJobTemplateName.ValueString(),
		OrganizationName:        m.OrganizationName.ValueString(),
		Limit:                   m.Limit.ValueString(),
	}

	jsonPayload, err := marshaler.Marshal(payload)
	if err != nil {
		var diags diag.Diagnostics
		diags.AddError(
			"Error marshaling event payload body",
			fmt.Sprintf("Unable to create event stream action event payload, unexpected error: %s", err.Error()),
		)
		return nil, diags
	}
	return jsonPayload, nil
}

// Create an http POST request to the configured Event Stream URL using basic auth
func (m *EventStreamActionModel) CreateRequest(ctx context.Context, body io.Reader) (*http.Request, diag.Diagnostics) {
	url := m.EventStreamConfig.Url.ValueString()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		var diags diag.Diagnostics
		diags.AddError(
			"Error creating request",
			fmt.Sprintf("Unable to create event stream action request, unexpected error: %s", err.Error()),
		)
		return nil, diags
	}

	const EDAEventStreamPostActionContentType = "application/json"
	req.Header.Set("Content-Type", EDAEventStreamPostActionContentType)

	// Only Basic auth supported at this time
	req.SetBasicAuth(m.EventStreamConfig.Username.ValueString(), m.EventStreamConfig.Password.ValueString())
	return req, nil
}

// Create an http.Client
func (m *EventStreamActionModel) CreateClient() *http.Client {
	insecureSkipVerify := m.EventStreamConfig.InsecureSkipVerify.ValueBool()
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: insecureSkipVerify},
	}
	client := &http.Client{Transport: tr}
	return client
}

type HttpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

func (a *EDAEventStreamPostAction) ExecuteRequest(client HttpClient, req *http.Request) ([]byte, diag.Diagnostics) {
	// Perform the request
	hresp, err := client.Do(req)
	if err != nil {
		var diags diag.Diagnostics
		diags.AddError(
			"Error executing request",
			fmt.Sprintf("Unable to execute event stream action request, unexpected error %s", err.Error()),
		)
		return nil, diags
	}
	// Close the response body when done
	defer hresp.Body.Close()

	// Handle the response
	body, err := io.ReadAll(hresp.Body)
	if err != nil {
		var diags diag.Diagnostics
		diags.AddError(
			"Error handling response",
			fmt.Sprintf("Unable to handle response from event stream action request, unexpected error %s", err.Error()),
		)
		return nil, diags
	}

	// Check the status code, should be created
	validStatusCodes := []int{
		http.StatusCreated,
		http.StatusOK,
	}
	if !slices.Contains(validStatusCodes, hresp.StatusCode) {
		var diags diag.Diagnostics
		diags.AddError(
			"Unexpected response status code",
			fmt.Sprintf("Received status code %v from event stream action request. Expecting one of %v, response body %q",
				hresp.StatusCode, validStatusCodes, string(body)),
		)
		return nil, diags
	}

	// OK we have a successful response
	return body, nil
}

// Invoke the action
func (a *EDAEventStreamPostAction) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var config EventStreamActionModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	jsonPayload, diags := config.CreateEventPayload()
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := config.EventStreamConfig.Url.ValueString()

	resp.SendProgress(action.InvokeProgressEvent{
		Message: fmt.Sprintf("Preparing event stream POST to %s", url),
	})

	// Create the request
	jsonPayloadReader := bytes.NewReader(jsonPayload)
	hreq, diags := config.CreateRequest(ctx, jsonPayloadReader)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	client := config.CreateClient()
	body, diags := a.ExecuteRequest(client, hreq)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.SendProgress(action.InvokeProgressEvent{
		Message: fmt.Sprintf("Sent event stream POST to %s, body %q", url, string(body)),
	})
}
