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

// ---------------------------------------------------------------------------
// BaseDetailDataSourceModel Adapter
// ---------------------------------------------------------------------------

func (o *BaseDetailDataSourceModel) CreateNamedURL(uri string, apiModel *BaseDetailAPIModel) (string, error) {
	return apiModel.CreateNamedURL(uri)
}

func (o *BaseDetailDataSourceModelWithOrg) CreateNamedURL(uri string, apiModel *BaseDetailAPIModelWithOrg) (string, error) {
	return apiModel.CreateNamedURL(uri)
}
