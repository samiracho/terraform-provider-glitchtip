// Copyright (c) samiracho
// SPDX-License-Identifier: MIT

package provider

import (
	"context"
	"fmt"
	"testing"

	fwdatasource "github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	r "github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// --- Unit tests ---

func TestOrganizationMembersDataSourceSchema(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	resp := &fwdatasource.SchemaResponse{}
	NewOrganizationMembersDataSource().Schema(ctx, fwdatasource.SchemaRequest{}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema method diagnostics: %+v", resp.Diagnostics)
	}
	if diags := resp.Schema.ValidateImplementation(ctx); diags.HasError() {
		t.Fatalf("schema validation diagnostics: %+v", diags)
	}
}

func TestOrganizationMembersItemFromAPI(t *testing.T) {
	t.Parallel()
	email := "owner@example.com"
	role := "owner"
	roleName := "Owner"
	dateCreated := "2026-06-20T00:00:00Z"
	got := organizationMembersItemFromAPI(organizationMembersListItem{
		ID: "42", Email: &email, Role: &role, RoleName: &roleName,
		Pending: false, IsOwner: true, DateCreated: &dateCreated,
	})
	if got.ID.ValueString() != "42" || got.Email.ValueString() != "owner@example.com" ||
		got.OrgRole.ValueString() != "owner" || got.RoleName.ValueString() != "Owner" ||
		got.Pending.ValueBool() || !got.IsOwner.ValueBool() ||
		got.DateCreated.ValueString() != "2026-06-20T00:00:00Z" {
		t.Fatalf("unexpected mapping: %+v", got)
	}

	// Null email/role/role_name/date_created map to null.
	gotNull := organizationMembersItemFromAPI(organizationMembersListItem{ID: "43"})
	if !gotNull.Email.IsNull() {
		t.Fatalf("nil email should map to null, got %q", gotNull.Email.ValueString())
	}
	if !gotNull.OrgRole.IsNull() {
		t.Fatalf("nil role should map to null, got %q", gotNull.OrgRole.ValueString())
	}
	if !gotNull.RoleName.IsNull() {
		t.Fatalf("nil role_name should map to null, got %q", gotNull.RoleName.ValueString())
	}
	if !gotNull.DateCreated.IsNull() {
		t.Fatalf("nil date_created should map to null, got %q", gotNull.DateCreated.ValueString())
	}
}

// --- Acceptance test ---

func TestAccOrganizationMembersDataSource_basic(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-org-members")

	r.Test(t, r.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []r.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`
resource "glitchtip_organization" "test" {
  name = %[1]q
}

data "glitchtip_organization_members" "all" {
  organization = glitchtip_organization.test.slug
  depends_on   = [glitchtip_organization.test]
}
`, rName),
				Check: r.ComposeAggregateTestCheckFunc(
					// A freshly-created org always has exactly one member: the owner.
					r.TestCheckResourceAttr("data.glitchtip_organization_members.all", "members.#", "1"),
					r.TestCheckResourceAttr("data.glitchtip_organization_members.all", "members.0.is_owner", "true"),
					r.TestCheckResourceAttrSet("data.glitchtip_organization_members.all", "members.0.email"),
				),
			},
		},
	})
}
