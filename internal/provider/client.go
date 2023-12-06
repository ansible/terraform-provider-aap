package provider

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
        "encoding/json"
	"fmt"
)

// Provider Http Client interface (will be useful for unit tests)
type ProviderHTTPClient interface {
	doRequest(method string, path string, data io.Reader) (int, []byte, error)
}

// Client -
type AAPClient struct {
	HostURL            string
	Username           *string
	Password           *string
	InsecureSkipVerify bool
        httpClient *http.Client
}

// ansible host
type AnsibleHost struct {
	Name      string            `json:"name"`
	Groups    []string          `json:"groups"`
	Variables map[string]string `json:"variables"`
}

// ansible host list
type AnsibleHostList struct {
	Hosts []AnsibleHost `json:"hosts"`
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
	client.httpClient = &http.Client{Transport: tr, Timeout: time.Duration(timeout) * time.Second}

	return &client, nil
}

func (c *AAPClient) GetHosts(stateId string) (*AnsibleHostList, error) {

	hostURL := c.HostURL
	if !strings.HasSuffix(hostURL, "/") {
		hostURL = hostURL + "/"
	}

	req, _ := http.NewRequest("GET", hostURL+"api/v2/state/"+stateId+"/", nil)
	if c.Username != nil && c.Password != nil {
		req.SetBasicAuth(*c.Username, *c.Password)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: c.InsecureSkipVerify},
	}
	client := &http.Client{Transport: tr}
	resp, err := client.Do(req)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status: %d, body: %s", resp.StatusCode, body)
	}

	return GetAnsibleHost(body)
}

func GetAnsibleHost(body []byte) (*AnsibleHostList, error) {

	var result map[string]interface{}
	err := json.Unmarshal(body, &result)
	if err != nil {
		return nil, err
	}

	var hosts AnsibleHostList
	resources, ok := result["resources"].([]interface{})
	if ok {
		for _, resource := range resources {
			resource_obj := resource.(map[string]interface{})
			resource_type, ok := resource_obj["type"]
			if ok && resource_type == "ansible_host" {
				instances, ok := resource_obj["instances"].([]interface{})
				if ok {
					for _, instance := range instances {
						attributes, ok := instance.(map[string]interface{})["attributes"].(map[string]interface{})
						if ok {
							name := attributes["name"].(string)
							var groups []string
							for _, group := range attributes["groups"].([]interface{}) {
								groups = append(groups, group.(string))
							}
							variables := make(map[string]string)
							for key, value := range attributes["variables"].(map[string]interface{}) {
								variables[key] = value.(string)
							}
							hosts.Hosts = append(hosts.Hosts, AnsibleHost{
								Name:      name,
								Groups:    groups,
								Variables: variables,
							})
						}
					}
				}
			}
		}
	}
	return &hosts, nil
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
