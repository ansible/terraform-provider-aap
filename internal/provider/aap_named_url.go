package provider

import (
	"errors"
	"fmt"
	"path"
	"strconv"
)

// ---------------------------------------------------------------------------
// Option 1: Attaching this function to our API Models
// ---------------------------------------------------------------------------

// Here for reference will be removed
// type BaseDetailAPIModel struct {
// 	Id            int64                 `json:"id"`
// 	Name          string                `json:"name,omitempty"`
// 	Description   string                `json:"description,omitempty"`
// 	URL           string                `json:"url"`
// 	Related       RelatedAPIModel       `json:"related"`
// 	SummaryFields SummaryFieldsAPIModel `json:"summary_fields"`
// 	Variables     string                `json:"variables,omitempty"`
// }

// type BaseDetailAPIModelWithOrg struct {
// 	BaseDetailAPIModel
// 	Organization int64 `json:"organization"`
// }

// IdOnlyStrategy equivalent would need the refactor of our API models to be complete.
// This helps show why Id and URL need to be pulled out.

func (o *BaseDetailAPIModel) CreateNamedURL(uri string) (string, error) {
	if o.Id != 0 {
		return path.Join(uri, strconv.FormatInt(o.Id, 10)), nil
	}
	if o.Name != "" {
		return path.Join(uri, o.Name), nil
	}

	return "", errors.New("invalid lookup parameters: id or name required")
}

func (o *BaseDetailAPIModelWithOrg) CreateNamedURL(uri string) (string, error) {
	if o.Id != 0 {
		return path.Join(uri, strconv.FormatInt(o.Id, 10)), nil
	}
	if o.Name != "" && o.SummaryFields.Organization.Name != "" {
		namedUrl := fmt.Sprintf("%s++%s", o.Name, o.SummaryFields.Organization.Name)
		return path.Join(uri, namedUrl), nil
	}

	return "", errors.New("invalid lookup parameters: id or [name and organization_name] required")
}

type urlOpts struct {
	CredentialTypeName      string
	CredentialTypeKind      string
	Hostname                string
	Id                      int64
	InventoryName           string
	Kind                    string
	Name                    string
	OrganizationName        string
	Username                string
	WorkflowJobTemplateName string
}

// ---------------------------------------------------------------------------
// Option 2: Strategy Pattern
// ---------------------------------------------------------------------------

// NamedURLStrategy defines the interface for creating named URLs
type NamedURLStrategy interface {
	CreateNamedURL(uri string, opts urlOpts) (string, error)
}

// BaseNamedURLStrategy provides common functionality
type BaseNamedURLStrategy struct{}

// IDOnlyStrategy creates URLs using only id
type IdOnlyStrategy struct {
	BaseNamedURLStrategy
}

// NameOnlyStrategy creates URLs using id or name
type IdOrNameStrategy struct {
	BaseNamedURLStrategy
}

type IdOrNameOrgStrategy struct {
	BaseNamedURLStrategy
}

func (s *BaseNamedURLStrategy) CreateNamedURL(uri string, opts urlOpts) (string, error) {
	return "", errors.New("CreateNamedURL must be implemented by concrete strategies")
}

func (s *IdOnlyStrategy) CreateNamedURL(uri string, opts urlOpts) (string, error) {
	if opts.Id != 0 {
		return path.Join(uri, strconv.FormatInt(opts.Id, 10)), nil
	}
	return "", errors.New("invalid lookup parameters: id required")
}

func (s *IdOrNameStrategy) CreateNamedURL(uri string, opts urlOpts) (string, error) {
	if opts.Id != 0 {
		return path.Join(uri, strconv.FormatInt(opts.Id, 10)), nil
	}
	if opts.Name != "" {
		return path.Join(uri, opts.Name), nil
	}
	return "", errors.New("invalid lookup parameters: id or name required")
}

func (s *IdOrNameOrgStrategy) CreateNamedURL(uri string, opts urlOpts) (string, error) {
	if opts.Id != 0 {
		return path.Join(uri, strconv.FormatInt(opts.Id, 10)), nil
	}
	if opts.Name != "" && opts.OrganizationName != "" {
		namedUrl := fmt.Sprintf("%s++%s", opts.Name, opts.OrganizationName)
		return path.Join(uri, namedUrl), nil
	}

	return "", errors.New("invalid lookup parameters: id or [name and organization_name] required")
}

// ---------------------------------------------------------------------------
// Factory Pattern - Functions
// ---------------------------------------------------------------------------

type namedUrlFunc func(uri string, opts urlOpts) (string, error)

func GetEndpointNamedUrl(apiEntitySlug string, uri string, opts urlOpts) (string, error) {
	return getEndpointNamedUrlFunc(apiEntitySlug, opts)(uri, opts)
}

func getEndpointNamedUrlFunc(apiEntitySlug string, _ urlOpts) namedUrlFunc {
	switch apiEntitySlug {
	case "organizations", "instance_groups":
		return nameNamedUrlFunc
	case "credential_types":
		return namekindNamedUrlFunc
	case "credentials":
		return namecredkindorgNamedUrlFunc
	case "teams", "notification_templates", "job_templates", "projects", "inventories",
		"inventory_scripts", "labels", "workflow_job_templates", "applications":
		return nameorgNamedUrlFunc
	case "hosts", "groups", "inventory_sources":
		return nameinvorgNamedUrlFunc
	case "workflow_job_templates_nodes":
		return idNamedUrlFunc
	case "users":
		return usernameNamedUrlFunc
	case "instances":
		return hostnameNamedUrlFunc
	default:
		return unsupportedNamedUrlFunc
	}
}

func unsupportedNamedUrlFunc(_ string, _ urlOpts) (string, error) {
	return "", errors.ErrUnsupported
}

func nameNamedUrlFunc(uri string, opts urlOpts) (string, error) {
	if opts.Id != 0 {
		return path.Join(uri, strconv.FormatInt(opts.Id, 10)), nil
	}
	if opts.Name != "" {
		return path.Join(uri, opts.Name), nil
	}

	return "", errors.New("invalid lookup parameters")
}

func namekindNamedUrlFunc(uri string, opts urlOpts) (string, error) {
	if opts.Id != 0 {
		return path.Join(uri, strconv.FormatInt(opts.Id, 10)), nil
	}
	if opts.Name != "" && opts.Kind != "" {
		namedUrl := fmt.Sprintf("%s++%s", opts.Name, opts.Kind)
		return path.Join(uri, namedUrl), nil
	}

	return "", errors.New("invalid lookup parameters")
}

func nameorgNamedUrlFunc(uri string, opts urlOpts) (string, error) {
	if opts.Id != 0 {
		return path.Join(uri, strconv.FormatInt(opts.Id, 10)), nil
	}
	if opts.Name != "" && opts.OrganizationName != "" {
		namedUrl := fmt.Sprintf("%s++%s", opts.Name, opts.OrganizationName)
		return path.Join(uri, namedUrl), nil
	}

	return "", errors.New("invalid lookup parameters")
}

func nameinvorgNamedUrlFunc(uri string, opts urlOpts) (string, error) {
	if opts.Id != 0 {
		return path.Join(uri, strconv.FormatInt(opts.Id, 10)), nil
	}
	if opts.Name != "" && opts.InventoryName != "" &&
		opts.OrganizationName != "" {
		namedUrl := fmt.Sprintf("%s++%s++%s", opts.Name, opts.InventoryName,
			opts.OrganizationName)
		return path.Join(uri, namedUrl), nil
	}

	return "", errors.New("invalid lookup parameters")
}

func namecredkindorgNamedUrlFunc(uri string, opts urlOpts) (string, error) {
	if opts.Id != 0 {
		return path.Join(uri, strconv.FormatInt(opts.Id, 10)), nil
	}
	if opts.Name != "" && opts.CredentialTypeName != "" && opts.CredentialTypeKind != "" && opts.OrganizationName != "" {
		namedUrl := fmt.Sprintf("%s++%s++%s++%s",
			opts.Name, opts.CredentialTypeName, opts.CredentialTypeKind, opts.OrganizationName)
		return path.Join(uri, namedUrl), nil
	}

	return "", errors.New("invalid lookup parameters")
}

func idNamedUrlFunc(uri string, opts urlOpts) (string, error) {
	if opts.Id != 0 {
		return path.Join(uri, strconv.FormatInt(opts.Id, 10)), nil
	}

	return "", errors.New("invalid lookup parameters")
}

func usernameNamedUrlFunc(uri string, opts urlOpts) (string, error) {
	if opts.Id != 0 {
		return path.Join(uri, strconv.FormatInt(opts.Id, 10)), nil
	}
	if opts.Username != "" {
		return path.Join(uri, opts.Username), nil
	}

	return "", errors.New("invalid lookup parameters")
}

func hostnameNamedUrlFunc(uri string, opts urlOpts) (string, error) {
	if opts.Id != 0 {
		return path.Join(uri, strconv.FormatInt(opts.Id, 10)), nil
	}
	if opts.Hostname != "" {
		return path.Join(uri, opts.Hostname), nil
	}

	return "", errors.New("invalid lookup parameters")
}
