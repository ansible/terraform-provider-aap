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
	var diags diag.Diagnostics

	createResponse, body, err := c.doRequest("POST", path, data)

	if err != nil {
		diags.AddError(
			fmt.Sprintf("Client request error, unable to create resource at path %s", path),
			err.Error(),
		)
		return nil, diags
	}
	if createResponse == nil {
		diags.AddError("HTTP response error", "No HTTP response from server")
		return nil, diags
	}
	if createResponse.StatusCode != http.StatusCreated {
		var info map[string]interface{}
		err := json.Unmarshal([]byte(body), &info)
		if err != nil {
			diags.AddError("Error unmarshaling response body", err.Error())
		}
		diags.AddError(
			fmt.Sprintf("Unexpected HTTP status code received while attempting to create resource at path %s", path),
			fmt.Sprintf("Expected (%d), got (%d). Response details: %v", http.StatusCreated, createResponse.StatusCode, info),
		)
		return nil, diags
	}
	return body, diags
}

// Get sends a GET request to the provided path, checks for errors, and returns the response body with any errors as diagnostics.
func (c *AAPClient) Get(path string) ([]byte, diag.Diagnostics) {
	var diags diag.Diagnostics

	getResponse, body, err := c.doRequest("GET", path, nil)

	if err != nil {
		diags.AddError(
			fmt.Sprintf("Client request error, unable to get resource at path %s", path),
			err.Error(),
		)
		return nil, diags
	}
	if getResponse == nil {
		diags.AddError("HTTP response error", "No HTTP response from server")
		return nil, diags
	}
	if getResponse.StatusCode != http.StatusOK {
		var info map[string]interface{}
		err := json.Unmarshal([]byte(body), &info)
		if err != nil {
			diags.AddError("Error unmarshaling response body", err.Error())
		}
		diags.AddError(
			fmt.Sprintf("Unexpected HTTP status code received while attempting to get resource at path %s", path),
			fmt.Sprintf("Expected (%d), got (%d). Response details: %v", http.StatusOK, getResponse.StatusCode, info),
		)
		return nil, diags
	}
	return body, diags
}

// Update sends a PUT request with the provided data to the provided path, checks for errors,
// and returns the response body with any errors as diagnostics.
func (c *AAPClient) Update(path string, data io.Reader) ([]byte, diag.Diagnostics) {
	var diags diag.Diagnostics

	updateResponse, body, err := c.doRequest("PUT", path, data)
	if err != nil {
		diags.AddError(
			fmt.Sprintf("Client request error, unable to update resource at path %s", path),
			err.Error(),
		)
		return nil, diags
	}
	if updateResponse == nil {
		diags.AddError("HTTP response error", "No HTTP response from server")
		return nil, diags
	}
	if updateResponse.StatusCode != http.StatusOK {
		var info map[string]interface{}
		err := json.Unmarshal([]byte(body), &info)
		if err != nil {
			diags.AddError("Error unmarshaling response body", err.Error())
		}
		diags.AddError(
			fmt.Sprintf("Unexpected HTTP status code received while attempting to update resource at path %s", path),
			fmt.Sprintf("Expected (%d), got (%d). Response details: %v", http.StatusCreated, updateResponse.StatusCode, info),
		)
		return nil, diags
	}

	return body, diags
}

// Delete sends a DELETE request to the provided path, checks for errors, and returns any errors as diagnostics.
func (c *AAPClient) Delete(path string) ([]byte, diag.Diagnostics) {
	var diags diag.Diagnostics

	deleteResponse, body, err := c.doRequest("DELETE", path, nil)
	if err != nil {
		diags.AddError(
			fmt.Sprintf("Client request error, unable to delete resource at path %s", path),
			err.Error(),
		)
		return nil, diags
	}
	if deleteResponse == nil {
		diags.AddError("HTTP response error", "No HTTP response from server")
		return nil, diags
	}
	if deleteResponse.StatusCode != http.StatusAccepted {
		var info map[string]interface{}
		err := json.Unmarshal([]byte(body), &info)
		if err != nil {
			diags.AddError("Error unmarshaling response body", err.Error())
		}
		diags.AddError(
			fmt.Sprintf("Unexpected HTTP status code received while attempting to delete resource at path %s", path),
			fmt.Sprintf("Expected (%d), got (%d). Response details: %v", http.StatusOK, deleteResponse.StatusCode, info),
		)
		return nil, diags
	}
	return body, diags
}
