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

To run acceptance tests locally, you will need a running instance of Ansible Automation Platform (AAP). You can either use an existing instance or deploy a local test environment using any supported method (e.g., containerized or VM-based deployment from official Red Hat resources).

Create an admin user for the AAP instance and set the following environment variables:

```bash
export AAP_USERNAME=<your admin username>
export AAP_PASSWORD=<your admin password>
export AAP_HOST="http://localhost:9080" # if using aap-dev (Note: Subject to change)
```

In order to run the acceptance tests for the job resource, you must have a working job template already in your AAP instance. The job template must be set to require an inventory on launch. Export the id of this job template:

```bash
export AAP_TEST_JOB_TEMPLATE_ID=<the ID of a job template in your AAP instance>
```

Then you can run acceptance tests with `make testacc`.

**WARNING**: running acceptance tests for the job resource will launch several jobs for the specified job template. It's strongly recommended that you create a "check" type job template for testing to ensure the launched jobs do not deploy any actual infrastructure.

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

1. Linux / AMD64

## Licensing

GNU General Public License v3.0. See [LICENSE](/LICENSE) for full text.
