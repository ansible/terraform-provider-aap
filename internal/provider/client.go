package provider

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
)

// Provider Http Client interface (will be useful for unit tests)
type ProviderHTTPClient interface {
	doRequest(method string, path string, data io.Reader) (*http.Response, []byte, error)
	Create(path string, data io.Reader) ([]byte, diag.Diagnostics)
	Get(path string) ([]byte, diag.Diagnostics)
	Update(path string, data io.Reader) ([]byte, diag.Diagnostics)
	Delete(path string) ([]byte, diag.Diagnostics)
}

// Client -
type AAPClient struct {
	HostURL    string
	Username   *string
	Password   *string
	httpClient *http.Client
}

// NewClient - create new AAPClient instance
func NewClient(host string, username *string, password *string, insecureSkipVerify bool, timeout int64) (*AAPClient, error) {
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

	return &client, nil
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
func (c *AAPClient) Get(path string) ([]byte, diag.Diagnostics) {
	getResponse, body, err := c.doRequest("GET", path, nil)
	diags := ValidateResponse(getResponse, body, err, []int{http.StatusOK})
	return body, diags
}

// Update sends a PUT request with the provided data to the provided path, checks for errors,
// and returns the response body with any errors as diagnostics.
func (c *AAPClient) Update(path string, data io.Reader) ([]byte, diag.Diagnostics) {
	updateResponse, body, err := c.doRequest("PUT", path, data)
	diags := ValidateResponse(updateResponse, body, err, []int{http.StatusOK})
	return body, diags
}

// Delete sends a DELETE request to the provided path, checks for errors, and returns any errors as diagnostics.
func (c *AAPClient) Delete(path string) ([]byte, diag.Diagnostics) {
	deleteResponse, body, err := c.doRequest("DELETE", path, nil)
	// Note: the AAP API documentation says that an inventory delete request should return a 204 response, but it currently returns a 202.
	// Once that bug is fixed we should be able to update this to just expect http.StatusNoContent.
	diags := ValidateResponse(deleteResponse, body, err, []int{http.StatusAccepted, http.StatusNoContent})
	return body, diags
}
