package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccInventoryResourceWithOrganizationDataSource(t *testing.T) {
	randomInventoryName := acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckInventoryResourceDestroy,
		Steps: []resource.TestStep{
			// Create an inventory using the organization data source
			{
				Config: createOrganizationAndInventory("Default", randomInventoryName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.aap_organization.default_org", "id", "1"),
					resource.TestCheckResourceAttr("data.aap_organization.default_org", "name", "Default"),
					// if this fails on 2.4 you need to add this description to the Default organization.
					resource.TestCheckResourceAttr("data.aap_organization.default_org", "description", "The default organization for Ansible Automation Platform"),
					resource.TestCheckResourceAttr("aap_inventory.new_inventory", "name", randomInventoryName),
					resource.TestCheckResourceAttr("data.aap_inventory.the_created_inventory", "name", randomInventoryName),
					resource.TestCheckResourceAttrPair("aap_inventory.new_inventory", "organization", "data.aap_inventory.the_created_inventory", "organization"),
					resource.TestCheckResourceAttrPair("aap_inventory.new_inventory", "description", "data.aap_inventory.the_created_inventory", "description"),
					resource.TestCheckResourceAttrPair("aap_inventory.new_inventory", "variables", "data.aap_inventory.the_created_inventory", "variables"),
					resource.TestCheckResourceAttrPair("aap_inventory.new_inventory", "url", "data.aap_inventory.the_created_inventory", "url"),
				),
			},
		},
	})
}

func createOrganizationAndInventory(organizationName string, inventoryName string) string {
	return fmt.Sprintf(`
data "aap_organization" "default_org" {
  name = "%s"
}

resource "aap_inventory" "new_inventory" {
  name        = "%s"
  organization = data.aap_organization.default_org.id
  description = "A test inventory"
}

data "aap_inventory" "the_created_inventory" {
  id = aap_inventory.new_inventory.id
}
`, organizationName, inventoryName)
}
