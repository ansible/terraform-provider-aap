package provider

var JobResponse1 = map[string]string{
	"status": "running",
	"type":   "check",
}

var JobResponse2 = map[string]string{
	"status":   "pending",
	"playbook": "ansible_aws.yaml",
}

var JobResponse3 = map[string]string{
	"status":                "complete",
	"execution_environment": "3",
}
var GroupResponse1 = map[string]string{
	"description": "",
	"inventory":   "1",
	"name":        "Group1",
}

var GroupResponse2 = map[string]string{
	"description": "Updated group",
	"inventory":   "1",
	"name":        "Group1",
}

var GroupResponse3 = map[string]string{
	"description": "",
	"inventory":   "3",
	"name":        "Group1",
	"variables":   "{\"ansible_network_os\": \"ios\"}",
}

var MockConfig = map[string]map[string]string{
	"/api/v2/job_templates/1/launch/": JobResponse1,
	"/api/v2/job_templates/2/launch/": JobResponse2,
	"/api/v2/jobs/1/":                 JobResponse1,
	"/api/v2/jobs/2/":                 JobResponse3,
	"/api/v2/groups/":                 GroupResponse1,
	"/api/v2/groups/1/":               GroupResponse2,
	"/api/v2/groups/2/":               GroupResponse3,
}
