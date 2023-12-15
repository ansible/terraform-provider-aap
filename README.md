# Terraform Provider for Ansible Automation Platform (AAP)

The AWS Provider allows Terraform to manage AAP resources.


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

### Testing

```shell
curl -L https://github.com/golangci/golangci-lint/releases/download/v1.50.1/golangci-lint-1.50.1-linux-amd64.tar.gz \
    | tar --wildcards -xzf - --strip-components 1 "**/golangci-lint"

# linters
make lint

# unit tests
make test

### Examples
The [examples](./examples/) subdirectory contains usage examples for this provider.

## Releasing

To release a new version of the provider:

1. Run `go generate` to regenerate docs
2. Commit changes
3. Push a new tag (this should trigger an automated release process to the Terraform Registry)
4. Verify the new version is published at https://registry.terraform.io/providers/ansible/aap/latest

## Licensing

GNU General Public License v3.0. See [LICENSE](/LICENSE) for full text.