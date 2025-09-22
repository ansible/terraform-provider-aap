package provider

// JobResponse1 is a test fixture for job responses
var JobResponse1 = map[string]string{
	"status": "running",
	"type":   "check",
}

// JobResponse2 is a test fixture for job responses
var JobResponse2 = map[string]string{
	"status":   "pending",
	"playbook": "ansible_aws.yaml",
}

// JobResponse3 is a test fixture for job responses
var JobResponse3 = map[string]string{
	"status":                "complete",
	"execution_environment": "3",
}

// GroupResponse1 is a test fixture for group responses
var GroupResponse1 = map[string]string{
	"description": "",
	"inventory":   "1",
	"name":        "Group1",
}

// GroupResponse2 is a test fixture for group responses
var GroupResponse2 = map[string]string{
	"description": "Updated group",
	"inventory":   "1",
	"name":        "Group1",
}

// GroupResponse3 is a test fixture for group responses
var GroupResponse3 = map[string]string{
	"description": "",
	"inventory":   "3",
	"name":        "Group1",
	"variables":   "{\"ansible_network_os\": \"ios\"}",
}

// MockConfig is a test fixture for mock configuration
var MockConfig = map[string]map[string]string{
	"/api/v2/job_templates/1/launch/": JobResponse1,
	"/api/v2/job_templates/2/launch/": JobResponse2,
	"/api/v2/jobs/1/":                 JobResponse1,
	"/api/v2/jobs/2/":                 JobResponse3,
	"/api/v2/groups/":                 GroupResponse1,
	"/api/v2/groups/1/":               GroupResponse2,
	"/api/v2/groups/2/":               GroupResponse3,
}
