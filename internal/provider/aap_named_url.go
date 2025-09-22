// Package provider implements named URLs for AAP API resources, allowing Terraform data sources
// to look up resources by human-readable names instead of just numeric IDs.
//
// Examples:
// - Standard URL: /api/v2/inventories/123/
// - Named URL:    /api/v2/inventories/MyInventory++MyOrg/
//
// Lookup patterns:
// - BaseDetailAPIModel: ID only
// - BaseDetailAPIModelWithOrg: ID OR (name + organization_name)
// - OrganizationAPIModel: ID OR name
//
// The "++" separator is AAP's standard format for name++organization lookups.
// ID lookup always takes precedence over name lookup for performance.
package provider

import (
	"errors"
	"fmt"
	"path"
	"strconv"
)

// ---------------------------------------------------------------------------
// BaseDetailAPIModel
// ---------------------------------------------------------------------------

// CreateNamedURL creates a named URL for BaseDetailAPIModel using ID only.
func (o *BaseDetailAPIModel) CreateNamedURL(uri string) (string, error) {
	if o.ID != 0 {
		return path.Join(uri, strconv.FormatInt(o.ID, 10)), nil
	}

	return "", errors.New("invalid lookup parameters: id required")
}

// CreateNamedURL creates a named URL for BaseDetailAPIModelWithOrg using ID or name+organization.
func (o *BaseDetailAPIModelWithOrg) CreateNamedURL(uri string) (string, error) {
	if o.ID != 0 {
		return path.Join(uri, strconv.FormatInt(o.ID, 10)), nil
	}
	if o.Name != "" && o.SummaryFields.Organization.Name != "" {
		namedURL := fmt.Sprintf("%s++%s", o.Name, o.SummaryFields.Organization.Name)
		return path.Join(uri, namedURL), nil
	}

	return "", errors.New("invalid lookup parameters: id or [name and organization_name] required")
}

// CreateNamedURL creates a named URL for OrganizationAPIModel using ID or name.
func (o *OrganizationAPIModel) CreateNamedURL(uri string) (string, error) {
	if o.ID != 0 {
		return path.Join(uri, strconv.FormatInt(o.ID, 10)), nil
	}
	if o.Name != "" {
		return path.Join(uri, o.Name), nil
	}

	return "", errors.New("invalid lookup parameters: id or name required")
}

// CreateNamedURL creates a named URL for BaseResourceAPIModel - always returns error as not supported.
func (o *BaseResourceAPIModel) CreateNamedURL(_ string) (string, error) {
	if o.URL != "" {
		return o.URL, nil
	}

	return "", errors.New("invalid lookup parameters: url required")
}

// ---------------------------------------------------------------------------
// BaseDetailDataSourceModel Adapter
// ---------------------------------------------------------------------------

// CreateNamedURL creates a named URL for BaseDetailSourceModel using the provided API model.
func (o *BaseDetailSourceModel) CreateNamedURL(uri string, apiModel *BaseDetailAPIModel) (string, error) {
	return apiModel.CreateNamedURL(uri)
}

// CreateNamedURL creates a named URL for BaseDetailSourceModelWithOrg using the provided API model.
func (o *BaseDetailSourceModelWithOrg) CreateNamedURL(uri string, apiModel *BaseDetailAPIModelWithOrg) (string, error) {
	return apiModel.CreateNamedURL(uri)
}

// CreateNamedURL creates a named URL for OrganizationDataSourceModel using the provided API model.
func (o *OrganizationDataSourceModel) CreateNamedURL(uri string, apiModel *OrganizationAPIModel) (string, error) {
	return apiModel.CreateNamedURL(uri)
}

// CreateNamedURL creates a named URL for BaseResourceSourceModel using the provided API model.
func (o *BaseResourceSourceModel) CreateNamedURL(uri string, apiModel *BaseResourceAPIModel) (string, error) {
	return apiModel.CreateNamedURL(uri)
}
