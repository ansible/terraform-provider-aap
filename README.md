# Terraform Provider for Ansible Automation Platform (AAP)

The AAP Provider allows Terraform to manage AAP resources. It provides a means of executing automation jobs on infrastructure provisioned by Terraform, leveraging the AAP API to manage inventories and launch jobs.

The provider can be found on [the Terraform registry](https://registry.terraform.io/providers/ansible/aap/latest).


## Requirements

- install Go: [official installation guide](https://go.dev/doc/install)
- install Terraform: [official installation guide](https://developer.hashicorp.com/terraform/tutorials/aws-get-started/install-cli)
- install AWX: [official installation guide](https://github.com/ansible/awx/blob/devel/INSTALL.md)

## Installation for Local Development

Run `make build`. This will build a `terraform-provider-aap` binary in the top level of the project. To get Terraform to use this binary, configure the [development overrides](https://developer.hashicorp.com/terraform/cli/config/config-file#development-overrides-for-provider-developers) for the provider installation. The easiest way to do this will be to create a config file with the following contents:

```
provider_installation {
  dev_overrides {
    "ansible/aap" = "/path/to/project/root"
  }

  direct {}
}
```

The `/path/to/project/root` should point to the location where you have cloned this repo, where the `terraform-provider-aap` binary will be built. You can then set the `TF_CLI_CONFIG_FILE` environment variable to point to this config file, and Terraform will use the provider binary you just built.

## Testing

### Linters
You will need to install [golangci-lint](https://golangci-lint.run/usage/install/) to run linters.

Run `make lint`

### Unit tests

Run `make test`

### Acceptance tests

Acceptance tests apply test terraform configurations to a running AAP instance and make changes to resources in that instance, use with caution!

To run acceptance tests locally, start a local AAP instance following the [docker-compose instructions for local AWX development](https://github.com/ansible/awx/blob/devel/tools/docker-compose/README.md). Create an admin user for the AAP instance and save the credentials to these environment variables:

Create an admin user for the AAP instance and set the following environment variables:

```bash
export AAP_USERNAME=<your admin username>
export AAP_PASSWORD=<your admin password>
export AAP_INSECURE_SKIP_VERIFY=true
export AAP_HOSTNAME=<your aap instance host url> # "http://localhost:9080" or "https://localhost:8043"
```

In order to run acceptance tests, there are multiple resources that must exist in AAP. While the provider can create some AAP resources, it is not designed for comprehensive management of all platform resources. We've added a playbook `testing/playbook.yml` to create the necessary resources to enable acceptance testing.

For example, the provider implements a `datasource.aap_organization` but does not implement a Terraform `resource` to create organizations. Executing the playbook creates `organization` and writes a file with `export AAP_TEST_ORGANIZATION_ID=#`.

The playbook uses modules from [console.redhat.com](https://console.redhat.com/ansible/automation-hub/repo/published/ansible/controller/). To configure `ansible-galaxy` to access this content, see https://access.redhat.com/solutions/6983440.

To install the collection and run the playbook:

```bash
# See https://access.redhat.com/solutions/6983440 to enable installation from console.redhat.com
ansible-galaxy collection install -r testing/requirements.yml
# AAP_USERNAME, AAP_PASSWORD, AAP_HOSTNAME must be set
# If you need to disable TLS verification, set AAP_VALIDATE_CERTS
export AAP_VALIDATE_CERTS=false
# For AAP 2.4, set CONTROLLER_OPTIONAL_API_URLPATTERN_PREFIX
export CONTROLLER_OPTIONAL_API_URLPATTERN_PREFIX="/api/"
ansible-playbook testing/playbook.yml
```

This will produce a file called `testing/acceptance_test_vars.env`. Source this file before running acceptance tests with `make testacc`.

```bash
source testing/acceptance_test_vars.env
make testacc
```

**WARNING**: running acceptance tests for the job resource will launch several jobs for the specified job template. Strongly recommended that you create a "check" type job template for testing to ensure the launched jobs do not deploy any actual infrastructure.

## Examples

The [examples](./examples/) subdirectory contains usage examples for this provider.

## Release notes

See the [generated changelog](https://github.com/ansible/terraform-provider-aap/tree/main/CHANGELOG.rst).

## Releasing

To release a new version of the provider:

1. Run `make generatedocs` to format the example files and regenerate docs using terraform-plugin-docs [tfplugindocs installation guide](https://github.com/hashicorp/terraform-plugin-docs?tab=readme-ov-file#installation).
2. Run `antsibull-changelog release --version <version>` to release a new version of the project.
3. Commit changes
4. Push a new tag (this should trigger an automated release process to the Terraform Registry). The tag version *must* start with "v", for example, v1.2.3.
5. Verify the new version is published at https://registry.terraform.io/providers/ansible/aap/latest

## Supported Platforms

1. Linux AMD64 and ARM64
2. Darwin AMD64 and ARM64

## Licensing

GNU General Public License v3.0. See [LICENSE](/LICENSE) for full text.
