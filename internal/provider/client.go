package provider

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
)

// Provider Http Client interface (will be useful for unit tests)
type ProviderHTTPClient interface {
	doRequest(method string, path string, data io.Reader) (*http.Response, []byte, error)
	Create(path string, data io.Reader) ([]byte, diag.Diagnostics)
	Get(path string) ([]byte, diag.Diagnostics)
	GetWithStatus(path string) ([]byte, diag.Diagnostics, int)
	Update(path string, data io.Reader) ([]byte, diag.Diagnostics)
	UpdateWithStatus(path string, data io.Reader) ([]byte, diag.Diagnostics, int)
	Delete(path string) ([]byte, diag.Diagnostics)
	DeleteWithStatus(path string) ([]byte, diag.Diagnostics, int)
	setApiEndpoint() diag.Diagnostics
	getApiEndpoint() string
}

// Client -
type AAPClient struct {
	HostURL     string
	Username    *string
	Password    *string
	httpClient  *http.Client
	ApiEndpoint string
}

type AAPApiEndpointResponse struct {
	Apis struct {
		Controller string `json:"controller"`
	} `json:"apis"`
	CurrentVersion string `json:"current_version"`
}

func readApiEndpoint(client ProviderHTTPClient) (string, diag.Diagnostics) {
	body, diags := client.Get("/api/")
	if diags.HasError() {
		return "", diags
	}
	var response AAPApiEndpointResponse
	err := json.Unmarshal(body, &response)
	if err != nil {
		diags.AddError(
			fmt.Sprintf("Unable to parse AAP API endpoint response: %s", string(body)),
			fmt.Sprintf("Unexpected error: %s", err.Error()),
		)
		return "", diags
	}
	if len(response.Apis.Controller) > 0 {
		body, diags = client.Get(response.Apis.Controller)
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
func NewClient(host string, username *string, password *string, insecureSkipVerify bool, timeout int64) (*AAPClient, diag.Diagnostics) {
	hostURL, _ := url.JoinPath(host, "/")
	client := AAPClient{
		HostURL:  hostURL,
		Username: username,
		Password: password,
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: insecureSkipVerify},
	}
	client.httpClient = &http.Client{Transport: tr, Timeout: time.Duration(timeout) * time.Second}

	// Set AAP API endpoint
	diags := client.setApiEndpoint()
	return &client, diags
}

func (c *AAPClient) setApiEndpoint() diag.Diagnostics {
	endpoint, diags := readApiEndpoint(c)
	if diags.HasError() {
		return diags
	}
	c.ApiEndpoint = endpoint
	return diags
}

func (c *AAPClient) getApiEndpoint() string {
	return c.ApiEndpoint
}

func (c *AAPClient) computeURLPath(path string) string {
	fullPath, _ := url.JoinPath(c.HostURL, path, "/")
	return fullPath
}

func (c *AAPClient) doRequest(method string, path string, data io.Reader) (*http.Response, []byte, error) {
	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, method, c.computeURLPath(path), data)
	if err != nil {
		return nil, []byte{}, err
	}
	if c.Username != nil && c.Password != nil {
		req.SetBasicAuth(*c.Username, *c.Password)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, []byte{}, err
	}

	defer resp.Body.Close()
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

// Get sends a GET request to the provided path, checks for errors, and returns the response body with any errors as diagnostics.
func (c *AAPClient) GetWithStatus(path string) ([]byte, diag.Diagnostics, int) {
	getResponse, body, err := c.doRequest("GET", path, nil)
	diags := ValidateResponse(getResponse, body, err, []int{http.StatusOK})
	if getResponse == nil {
		diags.AddError("HTTP response error", "No HTTP response from server")
		return body, diags, http.StatusInternalServerError
	}
	return body, diags, getResponse.StatusCode
}

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

// DeleteWithStatus sends a DELETE request to the provided path, checks for errors, and returns any errors as diagnostics and the status code.
func (c *AAPClient) DeleteWithStatus(path string) ([]byte, diag.Diagnostics, int) {
	deleteResponse, body, err := c.doRequest("DELETE", path, nil)
	// Note: the AAP API documentation says that an inventory delete request should return a 204 response, but it currently returns a 202.
	// Once that bug is fixed we should be able to update this to just expect http.StatusNoContent.
	diags := ValidateResponse(deleteResponse, body, err, []int{http.StatusAccepted, http.StatusNoContent})
	if deleteResponse == nil {
		diags.AddError("HTTP response error", "No HTTP response from server")
		return body, diags, http.StatusInternalServerError
	}
	return body, diags, deleteResponse.StatusCode
}

// Retry state constants
const (
	retryStateRetrying = "retrying"
	retryStateSuccess  = "success"
)

// Retry timing constants
const (
	maxTimeoutSeconds = 60 * 20 // Maximum wait between retries (seconds)
	minTimeoutSeconds = 5       // Minimum wait between retries (seconds)
	delaySeconds      = 30      // Initial delay before first retry (seconds)
	percentBuffer     = 0.2     // Percentage of remaining time to leave as buffer
)

// CreateRetryStateChangeConf creates a StateChangeConf for retrying operations with exponential backoff.
// This follows Terraform provider best practices for handling transient API errors.
//
// Retryable scenarios based on RFC 7231 and industry standards:
// - HTTP 409: Resource conflict (host in use by running jobs)
// - HTTP 408/429: Client timeouts and rate limiting
// - HTTP 5xx: Server-side transient errors
//
// The retry timeout is calculated from the context deadline, leaving a buffer to prevent conflicts.
func CreateRetryStateChangeConf(
	ctx context.Context,
	operation func() ([]byte, diag.Diagnostics, int),
	successStatusCodes []int,
	operationName string,
) *retry.StateChangeConf {
	// Calculate retry retryTimeout from context deadline
	retryTimeout := CalculateTimeout(ctx)

	stateConf := &retry.StateChangeConf{
		Pending: []string{retryStateRetrying},
		Target:  []string{retryStateSuccess},
		Refresh: func() (interface{}, string, error) {
			body, diags, statusCode := operation()

			// Check for retryable status codes
			switch statusCode {
			case http.StatusConflict:
				return nil, retryStateRetrying, nil // Keep retrying
			case http.StatusRequestTimeout, http.StatusTooManyRequests,
				http.StatusInternalServerError, http.StatusBadGateway,
				http.StatusServiceUnavailable, http.StatusGatewayTimeout:
				return nil, retryStateRetrying, nil // Keep retrying
			}

			// Check for success cases
			for _, successCode := range successStatusCodes {
				if statusCode == successCode {
					if diags.HasError() {
						return nil, "", fmt.Errorf("%s succeeded but diagnostics has errors: %v", operationName, diags)
					}
					return body, retryStateSuccess, nil
				}
			}

			// Non-retryable error
			return nil, "", fmt.Errorf("non-retryable HTTP status %d for %s", statusCode, operationName)
		},
		Timeout:    time.Duration(retryTimeout),
		MinTimeout: minTimeoutSeconds * time.Second, // Minimum wait between retries
		Delay:      delaySeconds * time.Second,      // Initial delay before first retry
	}

	return stateConf
}

// CalculateTimeout returns the retry timeout in seconds, which is 80% of the context timeout.
func CalculateTimeout(ctx context.Context) int {
	// Default fallback timeout in seconds
	timeout := maxTimeoutSeconds

	if deadline, ok := ctx.Deadline(); ok {
		remainingDuration := time.Until(deadline).Seconds()

		// If the deadline has already passed, use the minimum timeout
		if remainingDuration <= 0 {
			return minTimeoutSeconds
		}

		// Use 80% (1.0 - 0.20) of the remaining time for the timeout
		calculatedTimeoutSeconds := remainingDuration * (1.0 - percentBuffer)

		// Ensure the timeout is at least the minimum viable timeout
		if calculatedTimeoutSeconds < float64(minTimeoutSeconds) {
			timeout = minTimeoutSeconds
		} else {
			// Return the timeout as an integer, truncating any fractions of a second
			timeout = int(math.Round(calculatedTimeoutSeconds))
		}
	}
	return timeout
}
