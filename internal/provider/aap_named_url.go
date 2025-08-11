/*
AAP Named URL Creation

This file implements named URLs for AAP API resources, allowing Terraform data sources
to look up resources by human-readable names instead of just numeric IDs.

Examples:
- Standard URL: /api/v2/inventories/123/
- Named URL:    /api/v2/inventories/MyInventory++MyOrg/

Lookup patterns:
- BaseDetailAPIModel: ID only
- BaseDetailAPIModelWithOrg: ID OR (name + organization_name)
- OrganizationAPIModel: ID OR name

The "++" separator is AAP's standard format for name++organization lookups.
ID lookup always takes precedence over name lookup for performance.
*/
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

func (o *BaseDetailAPIModel) CreateNamedURL(uri string) (string, error) {
	if o.Id != 0 {
		return path.Join(uri, strconv.FormatInt(o.Id, 10)), nil
	}

	return "", errors.New("invalid lookup parameters: id required")
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

func (o *OrganizationAPIModel) CreateNamedURL(uri string) (string, error) {
	if o.Id != 0 {
		return path.Join(uri, strconv.FormatInt(o.Id, 10)), nil
	}
	if o.Name != "" {
		return path.Join(uri, o.Name), nil
	}

	return "", errors.New("invalid lookup parameters: id or name required")
}

func (o *BaseResourceAPIModel) CreateNamedURL(_ string) (string, error) {
	if o.Url != "" {
		return o.Url, nil
	}

	return "", errors.New("invalid lookup parameters: url required")
}

// ---------------------------------------------------------------------------
// BaseDetailDataSourceModel Adapter
// ---------------------------------------------------------------------------

func (o *BaseDetailSourceModel) CreateNamedURL(uri string, apiModel *BaseDetailAPIModel) (string, error) {
	return apiModel.CreateNamedURL(uri)
}

func (o *BaseDetailSourceModelWithOrg) CreateNamedURL(uri string, apiModel *BaseDetailAPIModelWithOrg) (string, error) {
	return apiModel.CreateNamedURL(uri)
}

func (o *OrganizationDataSourceModel) CreateNamedURL(uri string, apiModel *OrganizationAPIModel) (string, error) {
	return apiModel.CreateNamedURL(uri)
}

func (o *BaseResourceSourceModel) CreateNamedURL(uri string, apiModel *BaseResourceAPIModel) (string, error) {
	return apiModel.CreateNamedURL(uri)
}
