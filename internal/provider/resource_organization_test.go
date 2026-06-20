// Copyright (c) samiracho
// SPDX-License-Identifier: MIT

package provider

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	r "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/samiracho/terraform-provider-glitchtip/internal/client"
)

// --- Unit tests (no live instance required) ---

func TestOrganizationResourceSchema(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	resp := &fwresource.SchemaResponse{}
	NewOrganizationResource().Schema(ctx, fwresource.SchemaRequest{}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema method diagnostics: %+v", resp.Diagnostics)
	}
	if diags := resp.Schema.ValidateImplementation(ctx); diags.HasError() {
		t.Fatalf("schema validation diagnostics: %+v", diags)
	}
}

func TestOrganizationModelFromAPI(t *testing.T) {
	t.Parallel()
	out := organizationOut{ID: "7", Slug: "acme", Name: "Acme", DateCreated: "2026-06-20T00:00:00Z"}
	got := organizationModelFromAPI(out)
	if got.ID.ValueString() != "7" || got.Slug.ValueString() != "acme" ||
		got.Name.ValueString() != "Acme" || got.DateCreated.ValueString() != "2026-06-20T00:00:00Z" {
		t.Fatalf("unexpected model: %+v", got)
	}
}

func TestOrganizationPath(t *testing.T) {
	t.Parallel()
	if got := organizationPath("my org"); got != "/api/0/organizations/my%20org/" {
		t.Fatalf("organizationPath escaping = %q", got)
	}
}

// --- Acceptance tests (require TF_ACC=1 and a live GlitchTip instance) ---

func TestAccOrganizationResource_basic(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-org")
	rNameUpdated := rName + "-updated"

	r.Test(t, r.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckOrganizationDestroy,
		Steps: []r.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`
resource "glitchtip_organization" "test" {
  name = %[1]q
}
`, rName),
				Check: r.ComposeAggregateTestCheckFunc(
					r.TestCheckResourceAttr("glitchtip_organization.test", "name", rName),
					r.TestCheckResourceAttrSet("glitchtip_organization.test", "slug"),
					r.TestCheckResourceAttrSet("glitchtip_organization.test", "id"),
					r.TestCheckResourceAttrSet("glitchtip_organization.test", "date_created"),
				),
			},
			{
				ResourceName:      "glitchtip_organization.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: testAccOrganizationImportID,
			},
			{
				Config: providerConfig + fmt.Sprintf(`
resource "glitchtip_organization" "test" {
  name = %[1]q
}
`, rNameUpdated),
				Check: r.TestCheckResourceAttr("glitchtip_organization.test", "name", rNameUpdated),
			},
		},
	})
}

func testAccOrganizationImportID(s *terraform.State) (string, error) {
	rs, ok := s.RootModule().Resources["glitchtip_organization.test"]
	if !ok {
		return "", fmt.Errorf("organization resource not found in state")
	}
	return rs.Primary.Attributes["slug"], nil
}

func testAccCheckOrganizationDestroy(s *terraform.State) error {
	c := testAccClient()
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "glitchtip_organization" {
			continue
		}
		err := c.Do(context.Background(), http.MethodGet,
			organizationPath(rs.Primary.Attributes["slug"]), nil, nil)
		if client.IsNotFound(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("checking organization %s: %w", rs.Primary.Attributes["slug"], err)
		}
		return fmt.Errorf("organization %s still exists", rs.Primary.Attributes["slug"])
	}
	return nil
}
