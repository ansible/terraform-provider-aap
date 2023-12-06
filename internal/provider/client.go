package provider

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Provider Http Client interface (will be useful for unit tests)
type ProviderHTTPClient interface {
	doRequest(method string, path string, data io.Reader) (int, []byte, error)
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
	hostURL := host
	if !strings.HasSuffix(hostURL, "/") {
		hostURL += "/"
	}
	client := AAPClient{
		HostURL:  hostURL,
		Username: username,
		Password: password,
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: insecureSkipVerify},
	}
	// Initialize http client using default timeout = 5sec
	client.httpClient = &http.Client{Transport: tr, Timeout: time.Duration(timeout) * time.Second}

	return &client, nil
}

func (c *AAPClient) computeURLPath(path string) string {
	fullPath, _ := url.JoinPath(c.HostURL, path)
	if !strings.HasSuffix(fullPath, "/") {
		fullPath += "/"
	}
	return fullPath
}

func (c *AAPClient) doRequest(method string, path string, data io.Reader) (int, []byte, error) {
	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, method, c.computeURLPath(path), data)
	if err != nil {
		return -1, []byte{}, err
	}
	if c.Username != nil && c.Password != nil {
		req.SetBasicAuth(*c.Username, *c.Password)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return -1, []byte{}, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return -1, []byte{}, err
	}
	resp.Body.Close()
	return resp.StatusCode, body, nil
}
