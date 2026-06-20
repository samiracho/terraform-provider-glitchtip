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

func TestOrganizationMemberResourceSchema(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	resp := &fwresource.SchemaResponse{}
	NewOrganizationMemberResource().Schema(ctx, fwresource.SchemaRequest{}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema method diagnostics: %+v", resp.Diagnostics)
	}
	if diags := resp.Schema.ValidateImplementation(ctx); diags.HasError() {
		t.Fatalf("schema validation diagnostics: %+v", diags)
	}
}

func TestOrganizationMemberModelFromAPI(t *testing.T) {
	t.Parallel()

	out := organizationMemberOut{
		ID:          "42",
		Role:        "member",
		RoleName:    "Member",
		DateCreated: "2026-06-20T00:00:00Z",
		Email:       "person@example.com",
		Pending:     true,
		IsOwner:     false,
	}

	// organization and send_invite are not returned by the API and must be
	// carried through from plan/state.
	got := organizationMemberModelFromAPI(out, "acme", true)

	if got.ID.ValueString() != "42" {
		t.Fatalf("ID = %q, want %q", got.ID.ValueString(), "42")
	}
	if got.Organization.ValueString() != "acme" {
		t.Fatalf("Organization = %q, want %q (must be carried through)", got.Organization.ValueString(), "acme")
	}
	if got.OrgRole.ValueString() != "member" {
		t.Fatalf("OrgRole = %q, want %q (mapped from response.role)", got.OrgRole.ValueString(), "member")
	}
	if got.RoleName.ValueString() != "Member" {
		t.Fatalf("RoleName = %q, want %q", got.RoleName.ValueString(), "Member")
	}
	if got.DateCreated.ValueString() != "2026-06-20T00:00:00Z" {
		t.Fatalf("DateCreated = %q", got.DateCreated.ValueString())
	}
	if got.Email.ValueString() != "person@example.com" {
		t.Fatalf("Email = %q", got.Email.ValueString())
	}
	if got.Pending.ValueBool() != true {
		t.Fatalf("Pending = %v, want true", got.Pending.ValueBool())
	}
	if got.IsOwner.ValueBool() != false {
		t.Fatalf("IsOwner = %v, want false", got.IsOwner.ValueBool())
	}
	if got.SendInvite.ValueBool() != true {
		t.Fatalf("SendInvite = %v, want true (must be carried through)", got.SendInvite.ValueBool())
	}
}

func TestOrganizationMemberItemPath(t *testing.T) {
	t.Parallel()
	if got := organizationMemberItemPath("my org", "4 2"); got != "/api/0/organizations/my%20org/members/4%202/" {
		t.Fatalf("organizationMemberItemPath escaping = %q", got)
	}
	if got := organizationMemberPath("acme"); got != "/api/0/organizations/acme/members/" {
		t.Fatalf("organizationMemberPath = %q", got)
	}
}

// --- Acceptance tests (require TF_ACC=1 and a live GlitchTip instance) ---

func TestAccOrganizationMemberResource_basic(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-org")
	rEmail := fmt.Sprintf("%s@example.com", acctest.RandomWithPrefix("tf-acc"))

	r.Test(t, r.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckOrganizationMemberDestroy,
		Steps: []r.TestStep{
			{
				Config: testAccOrganizationMemberConfig(rName, rEmail, "member"),
				Check: r.ComposeAggregateTestCheckFunc(
					r.TestCheckResourceAttr("glitchtip_organization_member.test", "email", rEmail),
					r.TestCheckResourceAttr("glitchtip_organization_member.test", "org_role", "member"),
					r.TestCheckResourceAttrPair("glitchtip_organization_member.test", "organization",
						"glitchtip_organization.test", "slug"),
					r.TestCheckResourceAttrSet("glitchtip_organization_member.test", "id"),
					r.TestCheckResourceAttrSet("glitchtip_organization_member.test", "role_name"),
					r.TestCheckResourceAttrSet("glitchtip_organization_member.test", "pending"),
					r.TestCheckResourceAttrSet("glitchtip_organization_member.test", "is_owner"),
					r.TestCheckResourceAttrSet("glitchtip_organization_member.test", "date_created"),
				),
			},
			{
				ResourceName:            "glitchtip_organization_member.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateIdFunc:       testAccOrganizationMemberImportID,
				ImportStateVerifyIgnore: []string{"send_invite"},
			},
			{
				Config: testAccOrganizationMemberConfig(rName, rEmail, "admin"),
				Check:  r.TestCheckResourceAttr("glitchtip_organization_member.test", "org_role", "admin"),
			},
		},
	})
}

func testAccOrganizationMemberConfig(orgName, email, orgRole string) string {
	return providerConfig + fmt.Sprintf(`
resource "glitchtip_organization" "test" {
  name = %[1]q
}

resource "glitchtip_organization_member" "test" {
  organization = glitchtip_organization.test.slug
  email        = %[2]q
  org_role     = %[3]q
}
`, orgName, email, orgRole)
}

func testAccOrganizationMemberImportID(s *terraform.State) (string, error) {
	rs, ok := s.RootModule().Resources["glitchtip_organization_member.test"]
	if !ok {
		return "", fmt.Errorf("organization member resource not found in state")
	}
	return fmt.Sprintf("%s/%s",
		rs.Primary.Attributes["organization"],
		rs.Primary.Attributes["id"]), nil
}

func testAccCheckOrganizationMemberDestroy(s *terraform.State) error {
	c := testAccClient()
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "glitchtip_organization_member" {
			continue
		}
		err := c.Do(context.Background(), http.MethodGet,
			organizationMemberItemPath(rs.Primary.Attributes["organization"], rs.Primary.Attributes["id"]),
			nil, nil)
		if client.IsNotFound(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("checking organization member %s: %w", rs.Primary.Attributes["id"], err)
		}
		return fmt.Errorf("organization member %s still exists", rs.Primary.Attributes["id"])
	}
	return nil
}
