========================================
Terraform Provider for AAP Release Notes
========================================

.. contents:: Topics

v1.4.0
======

Major Changes
-------------

- Adds support for Token authentication via the AAP_TOKEN environment variable or 'token' attribute in the provider block.
- New Action 'aap_eda_eventstream_post' enabling action-based AAP integration via EDA Event Streams.
- New Action 'aap_job_launch' enabling action-based AAP Job launch.
- New Data Source 'aap_eda_eventstream' supporting lookup of EDA Event Streams for use with 'aap_eda_eventstream_post' Action.

Minor Changes
-------------

- In addition to the AAP_HOST environment variable, the provider now reads AAP_HOSTNAME to be consistent with other Ansible content. to be consistent with other Ansible content.

v1.3.0
======

Release Summary
---------------

Feature release

Major Changes
-------------

- New Datasource - aap_organization

Minor Changes
-------------

- Add Darwin and arm64 platform builds
- Adds support for Darwin arm64/amd64 and Linux arm64.
- Fixes issue where Inventory could become inconsistent.
- Update to Golang 1.23.9

Bugfixes
--------

- Fix issue with unknown values during plan phase.
- Prevent workflow job from using a default inventory id.
- datasource/base_datasource - Fixed an issue where unknown values were consider missing (#75)
- resource/aap_host - Deleting a host will be retried for a default of 30 minutes or until the job completion timeout has been reached (#68)
- resource/aap_job - A default inventory-id of 1 will no longer be enforced if a value is not present
- resource/aap_workflow_job - A default inventory-id of 1 will no longer be enforced if a value is not present (#111)
- resource/job - Jobs now correctly transition from pending to final states when using wait_for_completion = true (#78)

v1.2.0
======

Minor Changes
-------------

- Adds aap_job_template data source to support Job Templates.
- Adds workflow_job resource to support launching Workflow Jobs.
- Adds workflow_job_template data source to support Workflow Job Templates.
- Enhances aap_inventory data source to support looking up Inventory objects by their name and their organization name.
- Enhances aap_job resource to support waiting for the Job to complete before continuing.
- Support dynamic value for AAP endpoints since the value depends on the AAP version (https://github.com/ansible/terraform-provider-aap/pull/30).

Bugfixes
--------

- Fix plan failure when AAP job created by provider are deleted outside of terraform (https://github.com/ansible/terraform-provider-aap/pull/61).

v1.0.0
======

Release Summary
---------------

This is the initial release of the Terraform provider for AAP. The provider allows the user to create and manage AAP inventories, and launch job templates through Terraform.
