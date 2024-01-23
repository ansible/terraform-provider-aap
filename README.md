# Terraform Provider for Ansible Automation Platform (AAP)

The AAP Provider allows Terraform to manage AAP resources.


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

```bash
export AAP_USERNAME=<your admin username>
export AAP_PASSWORD=<your admin password>
```

Then you can run acceptance tests with `make testacc`.

Acceptance tests for the job resource will fail unless the following environment variables are also set:

```bash
export AAP_TEST_JOB_TEMPLATE_ID=<the ID of a job template in your AAP instance>
export AAP_TEST_JOB_INVENTORY_ID=<the ID of an inventory in your AAP instance>
```

**WARNING**: running acceptance tests for the job resource will launch several jobs for the specified job template. It's strongly recommended that you create a "check" type job template for testing to ensure the launched jobs do not deploy any actual infrastructure.

## Examples

The [examples](./examples/) subdirectory contains usage examples for this provider.

## Releasing

To release a new version of the provider:

1. Run `tfplugindocs generate` to regenerate docs [tfplugindocs installation guide](https://github.com/hashicorp/terraform-plugin-docs?tab=readme-ov-file#installation).
2. Commit changes
3. Push a new tag (this should trigger an automated release process to the Terraform Registry)
4. Verify the new version is published at https://registry.terraform.io/providers/ansible/aap/latest

## Licensing

GNU General Public License v3.0. See [LICENSE](/LICENSE) for full text.
