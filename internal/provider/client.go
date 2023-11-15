package provider

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Client -
type AAPClient struct {
	HostURL            string
	Username           *string
	Password           *string
	InsecureSkipVerify bool
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

// NewClient -
func NewClient(host string, username *string, password *string, insecure_skip_verify bool) (*AAPClient, error) {
	hostURL := host
	if !strings.HasSuffix(hostURL, "/") {
		hostURL = hostURL + "/"
	}
	client := AAPClient{
		HostURL:            hostURL,
		Username:           username,
		Password:           password,
		InsecureSkipVerify: insecure_skip_verify,
	}

	return &client, nil
}

func (c *AAPClient) computeURLPath(path string) string {
	fullPath := c.HostURL
	if strings.HasPrefix(path, "/") {
		fullPath = fullPath + path[1:]
	} else {
		fullPath = fullPath + path
	}
	if !strings.HasSuffix(fullPath, "/") {
		fullPath = fullPath + "/"
	}
	return fullPath
}

func (c *AAPClient) SendRequest(req *http.Request) (int, []byte, error) {
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
		return -1, []byte{}, err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return -1, []byte{}, err
	}
	return resp.StatusCode, body, nil
}

func (c *AAPClient) doRequestWithBody(method string, path string, req_data *bytes.Reader) (int, []byte, error) {

	req, _ := http.NewRequest(method, c.computeURLPath(path), req_data)
	return c.SendRequest(req)
}

func (c *AAPClient) doRequest(method string, path string) (int, []byte, error) {

	req, _ := http.NewRequest(method, c.computeURLPath(path), nil)
	return c.SendRequest(req)
}

func (c *AAPClient) GetHosts(stateId string) (*AnsibleHostList, error) {

	req, _ := http.NewRequest("GET", c.HostURL+"api/v2/state/"+stateId+"/", nil)

	http_status_code, body, err := c.SendRequest(req)
	if err != nil {
		return nil, err
	}

	if http_status_code != http.StatusOK {
		return nil, fmt.Errorf("status: %d, body: %s", http_status_code, body)
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
