package provider

import (
	"errors"
	"fmt"
	"path"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

type urlOpts struct {
	CredentialTypeName      types.String
	CredentialTypeKind      types.String
	Hostname                types.String
	Id                      types.Int64
	InventoryName           types.String
	Kind                    types.String
	Name                    types.String
	OrganizationName        types.String
	Username                types.String
	WorkflowJobTemplateName types.String
}

func (o *urlOpts) Equals(cmp *urlOpts) bool {
	if IsValueProvided(o.CredentialTypeName) && !o.CredentialTypeName.Equal(cmp.CredentialTypeName) {
		return false
	}
	if IsValueProvided(o.CredentialTypeKind) && !o.CredentialTypeKind.Equal(cmp.CredentialTypeKind) {
		return false
	}
	if IsValueProvided(o.Hostname) && !o.Hostname.Equal(cmp.Hostname) {
		return false
	}
	if IsValueProvided(o.Id) && !o.Id.Equal(cmp.Id) {
		return false
	}
	if IsValueProvided(o.InventoryName) && !o.InventoryName.Equal(cmp.InventoryName) {
		return false
	}
	if IsValueProvided(o.Kind) && !o.Kind.Equal(cmp.Kind) {
		return false
	}
	if IsValueProvided(o.Name) && !o.Name.Equal(cmp.Name) {
		return false
	}
	if IsValueProvided(o.OrganizationName) && !o.OrganizationName.Equal(cmp.OrganizationName) {
		return false
	}
	if IsValueProvided(o.Username) && !o.Username.Equal(cmp.Username) {
		return false
	}
	if IsValueProvided(o.WorkflowJobTemplateName) && !o.WorkflowJobTemplateName.Equal(cmp.WorkflowJobTemplateName) {
		return false
	}

	return true
}

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
	if IsValueProvided(opts.Id) {
		return path.Join(uri, opts.Id.String()), nil
	}
	if IsValueProvided(opts.Name) {
		return path.Join(uri, opts.Name.ValueString()), nil
	}

	return "", errors.New("invalid lookup parameters")
}

func namekindNamedUrlFunc(uri string, opts urlOpts) (string, error) {
	if IsValueProvided(opts.Id) {
		return path.Join(uri, opts.Id.String()), nil
	}
	if IsValueProvided(opts.Name) && IsValueProvided(opts.Kind) {
		namedUrl := fmt.Sprintf("%s++%s", opts.Name.ValueString(), opts.Kind.ValueString())
		return path.Join(uri, namedUrl), nil
	}

	return "", errors.New("invalid lookup parameters")
}

func nameorgNamedUrlFunc(uri string, opts urlOpts) (string, error) {
	if IsValueProvided(opts.Id) {
		return path.Join(uri, opts.Id.String()), nil
	}
	if IsValueProvided(opts.Name) && IsValueProvided(opts.OrganizationName) {
		namedUrl := fmt.Sprintf("%s++%s", opts.Name.ValueString(), opts.OrganizationName.ValueString())
		return path.Join(uri, namedUrl), nil
	}

	return "", errors.New("invalid lookup parameters")
}

func nameinvorgNamedUrlFunc(uri string, opts urlOpts) (string, error) {
	if IsValueProvided(opts.Id) {
		return path.Join(uri, opts.Id.String()), nil
	}
	if IsValueProvided(opts.Name) && IsValueProvided(opts.InventoryName) &&
		IsValueProvided(opts.OrganizationName) {
		namedUrl := fmt.Sprintf("%s++%s++%s", opts.Name.ValueString(), opts.InventoryName.ValueString(),
			opts.OrganizationName.ValueString())
		return path.Join(uri, namedUrl), nil
	}

	return "", errors.New("invalid lookup parameters")
}

func namecredkindorgNamedUrlFunc(uri string, opts urlOpts) (string, error) {
	if IsValueProvided(opts.Id) {
		return path.Join(uri, opts.Id.String()), nil
	}
	if IsValueProvided(opts.Name) && IsValueProvided(opts.CredentialTypeName) &&
		IsValueProvided(opts.CredentialTypeKind) && IsValueProvided(opts.OrganizationName) {
		namedUrl := fmt.Sprintf("%s++%s++%s++%s",
			opts.Name.ValueString(), opts.CredentialTypeName.ValueString(),
			opts.CredentialTypeKind.ValueString(), opts.OrganizationName.ValueString())
		return path.Join(uri, namedUrl), nil
	}

	return "", errors.New("invalid lookup parameters")
}

func idNamedUrlFunc(uri string, opts urlOpts) (string, error) {
	if IsValueProvided(opts.Id) {
		return path.Join(uri, opts.Id.String()), nil
	}

	return "", errors.New("invalid lookup parameters")
}

func usernameNamedUrlFunc(uri string, opts urlOpts) (string, error) {
	if IsValueProvided(opts.Id) {
		return path.Join(uri, opts.Id.String()), nil
	}
	if IsValueProvided(opts.Username) {
		return path.Join(uri, opts.Username.ValueString()), nil
	}

	return "", errors.New("invalid lookup parameters")
}

func hostnameNamedUrlFunc(uri string, opts urlOpts) (string, error) {
	if IsValueProvided(opts.Id) {
		return path.Join(uri, opts.Id.String()), nil
	}
	if IsValueProvided(opts.Hostname) {
		return path.Join(uri, opts.Hostname.ValueString()), nil
	}

	return "", errors.New("invalid lookup parameters")
}
