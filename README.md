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
export AAP_HOST=<your aap instance host url> # "http://localhost:9080" or "https://localhost:8043"
```

In order to run the acceptance tests for the job resource, you must have templates for job and worklow already in your AAP instance. The templates must be set to require an inventory on launch and the Workflow Template must be named "Demo Workflow Job Template". Then associate the Workflow Template with the "Default" organization.

Export the IDs of these job templates:

```bash
export AAP_TEST_JOB_TEMPLATE_ID=<the ID of a job template in your AAP instance>
export AAP_TEST_WORKFLOW_JOB_TEMPLATE_ID=<the ID of a workflow job template in your AAP instance>
```

The inventory resource test requires the AAP instance to have a second organization with the name `Non-Default` and export that ID:

```bash
export AAP_TEST_ORGANIZATION_ID=<the ID of Non-Default in your AAP instance>
```
The Workflow Job Template resource test requires the AAP instance to have an inventory named "Inventory For Workflow" and then a Workflow Job Template named "Workflow with Inventory".  Follow the below steps to get the data in AAP setup:

1. Create inventory `Inventory For Workflow` on Default organization - make note of the ID for the Inventory
2. Create a Workflow Job Template called `Workflow with Inventory` - make note of the ID of the Workflow Job Template.
  - Assign organization to `Default`
  - Assign `Inventory For Workflow`
  - Make sure `Prompt on launch` **is not checked** for the inventory
  - Make sure `Prompt on launch` **is checked** for `Extra variables`
  - Add a default step and save

Export the following environment variables using the IDs from above:

```bash
export AAP_TEST_WORKFLOW_INVENTORY_ID=<the ID of `Workflow with Inventory`>
export AAP_TEST_INVENTORY_FOR_WF_ID=<the ID of `Inventory For Workflow`>
```

AAP 2.4 version note - If you are running the tests against an AAP 2.4 version instance, set the description for Default Organization to `The default organization for Ansible Automation Platform`

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

1. Linux AMD64 and ARM64
2. Darwin AMD64 and ARM64

## Licensing

GNU General Public License v3.0. See [LICENSE](/LICENSE) for full text.
