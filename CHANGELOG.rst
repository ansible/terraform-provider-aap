========================================
Terraform Provider for AAP Release Notes
========================================

.. contents:: Topics

v1.3.0-prerelease2
==================

v1.3.0-prerelease
=================

Release Summary
---------------

Feature release

Minor Changes
-------------

- Adds support for Darwin arm64/amd64 and Linux arm64.
- Fixes issue where Inventory could become inconsistent.

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
