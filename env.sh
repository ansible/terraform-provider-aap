export AAP_USERNAME=admin
export AAP_PASSWORD=<from aap-dev make admin-password>
export AAP_HOST=http://localhost:44925
export AAP_TEST_JOB_TEMPLATE_ID=7 #Set Inventory Prompt On Launch on 7=Demo Job Template
export AAP_TEST_WORKFLOW_JOB_TEMPLATE_ID=8
export AAP_TEST_ORGANIZATION_ID=2
export TF_ACC=1

# TODO: Makefile to build these out automatically.in aap-dev to build these. This was de-prioritized. see https://issues.redhat.com/browse/AAP-44341
export AAP_TEST_ORGANIZATION_NAME=Non-Default
export AAP_TEST_WORKFLOW_JOB_TEMPLATE_ID="Demo Workflow Job Template" # needs inventory prompt on launch and default or nd demo inventory and a step with default job template.