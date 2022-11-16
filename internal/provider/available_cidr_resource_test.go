package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccExampleResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccExampleResourceConfig([]string{"10.1.0.0/16"}, []string{"10.1.0.0/24"}, 24),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("utility_available_cidr.test", "result", "10.1.1.0/24"),
					resource.TestCheckResourceAttr("utility_available_cidr.test", "id", "10.1.1.0/24"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "utility_available_cidr.test",
				ImportState:       true,
				ImportStateVerify: true,
				// This is not normally necessary, but is here because this
				// example code does not have an actual upstream service.
				// Once the Read method is able to refresh information from
				// the upstream service, this can be removed.
				ImportStateVerifyIgnore: []string{"from_cidrs", "used_cidrs"},
			},
			// Update and Read testing
			{
				Config: testAccExampleResourceConfig([]string{"10.0.0.0/16"}, []string{"10.0.0.0/24"}, 24),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("utility_available_cidr.test", "result", "10.1.1.0/24"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccExampleResourceConfig(from []string, used []string, mask int) string {
	return fmt.Sprintf(`
resource "utility_available_cidr" "test" {
  from_cidrs = %q
  used_cidrs = %q
  mask = %v
}
`, from, used, mask)
}
