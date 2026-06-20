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

func TestTeamsDataSourceSchema(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	resp := &fwdatasource.SchemaResponse{}
	NewTeamsDataSource().Schema(ctx, fwdatasource.SchemaRequest{}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema method diagnostics: %+v", resp.Diagnostics)
	}
	if diags := resp.Schema.ValidateImplementation(ctx); diags.HasError() {
		t.Fatalf("schema validation diagnostics: %+v", diags)
	}
}

func TestTeamsItemFromAPI(t *testing.T) {
	t.Parallel()
	slug := "ops"
	memberCount := int64(3)
	isMember := true
	got := teamsItemFromAPI(teamsListItem{
		ID: "7", Slug: &slug, DateCreated: "2026-06-20T00:00:00Z",
		MemberCount: &memberCount, IsMember: &isMember,
	})
	if got.ID.ValueString() != "7" || got.Slug.ValueString() != "ops" ||
		got.DateCreated.ValueString() != "2026-06-20T00:00:00Z" ||
		got.MemberCount.ValueInt64() != 3 {
		t.Fatalf("unexpected mapping: %+v", got)
	}

	// Null slug/member_count map to null.
	gotNull := teamsItemFromAPI(teamsListItem{ID: "8"})
	if !gotNull.Slug.IsNull() {
		t.Fatalf("nil slug should map to null, got %q", gotNull.Slug.ValueString())
	}
	if !gotNull.MemberCount.IsNull() {
		t.Fatalf("nil memberCount should map to null, got %d", gotNull.MemberCount.ValueInt64())
	}
}

// --- Acceptance test ---

func TestAccTeamsDataSource_basic(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-teams")

	r.Test(t, r.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []r.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`
resource "glitchtip_organization" "test" {
  name = %[1]q
}

resource "glitchtip_team" "a" {
  organization = glitchtip_organization.test.slug
  slug         = "%[1]s-a"
}

resource "glitchtip_team" "b" {
  organization = glitchtip_organization.test.slug
  slug         = "%[1]s-b"
}

data "glitchtip_teams" "all" {
  organization = glitchtip_organization.test.slug
  depends_on   = [glitchtip_team.a, glitchtip_team.b]
}
`, rName),
				Check: r.ComposeAggregateTestCheckFunc(
					// The freshly-created org has exactly the two teams.
					r.TestCheckResourceAttr("data.glitchtip_teams.all", "teams.#", "2"),
					r.TestCheckResourceAttrSet("data.glitchtip_teams.all", "teams.0.id"),
					r.TestCheckResourceAttrSet("data.glitchtip_teams.all", "teams.0.slug"),
				),
			},
		},
	})
}
