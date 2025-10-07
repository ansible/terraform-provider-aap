package provider

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
)

// HTTPClient interface provides HTTP client functionality for the provider.
type HTTPClient interface {
	doRequest(method string, path string, data io.Reader) (*http.Response, []byte, error)
	Create(path string, data io.Reader) ([]byte, diag.Diagnostics)
	Get(path string) ([]byte, diag.Diagnostics)
	GetWithStatus(path string) ([]byte, diag.Diagnostics, int)
	Update(path string, data io.Reader) ([]byte, diag.Diagnostics)
	UpdateWithStatus(path string, data io.Reader) ([]byte, diag.Diagnostics, int)
	Delete(path string) ([]byte, diag.Diagnostics)
	DeleteWithStatus(path string) ([]byte, diag.Diagnostics, int)
	setAPIEndpoint() diag.Diagnostics
	getAPIEndpoint() string
}

// AAPClient provides functionality for interacting with the AAP API.
type AAPClient struct {
	HostURL       string
	Authenticator AAPClientAuthenticator
	httpClient    *http.Client
	APIEndpoint   string
}

// AAPAPIEndpointResponse represents a response from an AAP API endpoint.
type AAPAPIEndpointResponse struct {
	APIs struct {
		Controller string `json:"controller"`
	} `json:"apis"`
	CurrentVersion string `json:"current_version"`
}

func readAPIEndpoint(client HTTPClient) (string, diag.Diagnostics) {
	body, diags := client.Get("/api/")
	if diags.HasError() {
		return "", diags
	}
	var response AAPAPIEndpointResponse
	err := json.Unmarshal(body, &response)
	if err != nil {
		diags.AddError(
			fmt.Sprintf("Unable to parse AAP API endpoint response: %s", string(body)),
			fmt.Sprintf("Unexpected error: %s", err.Error()),
		)
		return "", diags
	}
	if len(response.APIs.Controller) > 0 {
		body, diags = client.Get(response.APIs.Controller)
		if diags.HasError() {
			return "", diags
		}
		// Parse response
		err = json.Unmarshal(body, &response)
		if err != nil {
			diags.AddError(
				fmt.Sprintf("Unable to parse AAP API endpoint response: %s", string(body)),
				fmt.Sprintf("Unexpected error: %s", err.Error()),
			)
			return "", diags
		}
	}
	if len(response.CurrentVersion) == 0 {
		diags.AddError("Unable to determine API Endpoint", "The controller endpoint is missing from response")
		return "", diags
	}
	return response.CurrentVersion, diags
}

// NewClient - create new AAPClient instance
func NewClient(host string, authenticator AAPClientAuthenticator, insecureSkipVerify bool, timeout int64) (
	*AAPClient, diag.Diagnostics) {
	hostURL, _ := url.JoinPath(host, "/")
	client := AAPClient{
		HostURL:       hostURL,
		Authenticator: authenticator,
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: insecureSkipVerify}, // User configurable option
	}
	client.httpClient = &http.Client{Transport: tr, Timeout: time.Duration(timeout) * time.Second}

	// Set AAP API endpoint
	diags := client.setAPIEndpoint()
	return &client, diags
}

func (c *AAPClient) setAPIEndpoint() diag.Diagnostics {
	endpoint, diags := readAPIEndpoint(c)
	if diags.HasError() {
		return diags
	}
	c.APIEndpoint = endpoint
	return diags
}

func (c *AAPClient) getAPIEndpoint() string {
	return c.APIEndpoint
}

func (c *AAPClient) computeURLPath(path string) string {
	// Parse the input path to separate path and query
	u, err := url.Parse(path)
	if err != nil {
		// If parsing fails, fallback to joining as is
		fullPath, _ := url.JoinPath(c.HostURL, path, "/")
		return fullPath
	}
	// Join only the Path part with HostURL
	fullPath, _ := url.JoinPath(c.HostURL, u.Path, "/")

	// Reattach query if exists
	if u.RawQuery != "" {
		fullPath = fullPath + "?" + u.RawQuery
	}
	return fullPath
}

func (c *AAPClient) doRequest(method string, path string, data io.Reader) (*http.Response, []byte, error) {
	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, method, c.computeURLPath(path), data)
	if err != nil {
		return nil, []byte{}, err
	}
	if c.Authenticator != nil {
		c.Authenticator.Configure(req)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, []byte{}, err
	}

	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, []byte{}, err
	}
	return resp, body, nil
}

// Create sends a POST request with the provided data to the provided path, checks for errors,
// and returns the response body with any errors as diagnostics.
func (c *AAPClient) Create(path string, data io.Reader) ([]byte, diag.Diagnostics) {
	createResponse, body, err := c.doRequest("POST", path, data)
	diags := ValidateResponse(createResponse, body, err, []int{http.StatusCreated})
	return body, diags
}

// GetWithStatus sends a GET request to the provided path and returns the response body, diagnostics, and status code.
func (c *AAPClient) GetWithStatus(path string) ([]byte, diag.Diagnostics, int) {
	getResponse, body, err := c.doRequest("GET", path, nil)
	diags := ValidateResponse(getResponse, body, err, []int{http.StatusOK})
	if getResponse == nil {
		diags.AddError("HTTP response error", "No HTTP response from server")
		return body, diags, http.StatusInternalServerError
	}
	return body, diags, getResponse.StatusCode
}

// Get sends a GET request to the provided path and returns the response body with any errors as diagnostics.
func (c *AAPClient) Get(path string) ([]byte, diag.Diagnostics) {
	body, diags, _ := c.GetWithStatus(path)
	return body, diags
}

// Update sends a PUT request with the provided data to the provided path, checks for errors,
// and returns the response body with any errors as diagnostics.
func (c *AAPClient) Update(path string, data io.Reader) ([]byte, diag.Diagnostics) {
	body, diags, _ := c.UpdateWithStatus(path, data)
	return body, diags
}

// UpdateWithStatus sends a PUT request with the provided data to the provided path, checks for errors,
// and returns the response body with any errors as diagnostics and the status code.
func (c *AAPClient) UpdateWithStatus(path string, data io.Reader) ([]byte, diag.Diagnostics, int) {
	updateResponse, body, err := c.doRequest("PUT", path, data)
	diags := ValidateResponse(updateResponse, body, err, []int{http.StatusOK})
	if updateResponse == nil {
		diags.AddError("HTTP response error", "No HTTP response from server")
		return body, diags, http.StatusInternalServerError
	}
	return body, diags, updateResponse.StatusCode
}

// Delete sends a DELETE request to the provided path, checks for errors, and returns any errors as diagnostics.
func (c *AAPClient) Delete(path string) ([]byte, diag.Diagnostics) {
	body, diags, _ := c.DeleteWithStatus(path)
	return body, diags
}

// DeleteWithStatus sends a DELETE request to the provided path, checks for errors,
// and returns any errors as diagnostics and the status code.
func (c *AAPClient) DeleteWithStatus(path string) ([]byte, diag.Diagnostics, int) {
	deleteResponse, body, err := c.doRequest("DELETE", path, nil)
	// Note: the AAP API documentation says that an inventory delete request should return a 204 response,
	// but it currently returns a 202.
	// Once that bug is fixed we should be able to update this to just expect http.StatusNoContent.
	diags := ValidateResponse(deleteResponse, body, err, []int{http.StatusAccepted, http.StatusNoContent})
	if deleteResponse == nil {
		diags.AddError("HTTP response error", "No HTTP response from server")
		return body, diags, http.StatusInternalServerError
	}
	return body, diags, deleteResponse.StatusCode
}
