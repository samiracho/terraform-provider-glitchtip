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

// --- Unit tests (no live instance required) ---

func TestTeamDataSourceSchema(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	resp := &fwdatasource.SchemaResponse{}
	NewTeamDataSource().Schema(ctx, fwdatasource.SchemaRequest{}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema method diagnostics: %+v", resp.Diagnostics)
	}
	if diags := resp.Schema.ValidateImplementation(ctx); diags.HasError() {
		t.Fatalf("schema validation diagnostics: %+v", diags)
	}
}

func TestTeamDataSourceModelFromAPI(t *testing.T) {
	t.Parallel()
	out := teamDataSourceOut{
		ID:          "42",
		Slug:        "engineering",
		DateCreated: "2026-06-20T00:00:00Z",
		IsMember:    true,
		MemberCount: 3,
	}
	got := teamDataSourceModelFromAPI(out, "acme")
	if got.ID.ValueString() != "42" || got.Slug.ValueString() != "engineering" ||
		got.DateCreated.ValueString() != "2026-06-20T00:00:00Z" {
		t.Fatalf("unexpected model: %+v", got)
	}
	// organization is not returned by the API and must be carried through.
	if got.Organization.ValueString() != "acme" {
		t.Fatalf("organization passthrough = %q, want %q", got.Organization.ValueString(), "acme")
	}
	if got.MemberCount.ValueInt64() != 3 {
		t.Fatalf("member_count mapping = %d, want 3", got.MemberCount.ValueInt64())
	}
}

func TestTeamDataSourcePath(t *testing.T) {
	t.Parallel()
	if got := teamDataSourcePath("my org", "my team"); got != "/api/0/teams/my%20org/my%20team/" {
		t.Fatalf("teamDataSourcePath escaping = %q", got)
	}
}

// --- Acceptance tests (require TF_ACC=1 and a live GlitchTip instance) ---

func TestAccTeamDataSource_basic(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-org")
	rTeam := acctest.RandomWithPrefix("tf-acc-team")

	r.Test(t, r.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []r.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`
resource "glitchtip_organization" "test" {
  name = %[1]q
}

resource "glitchtip_team" "test" {
  organization = glitchtip_organization.test.slug
  slug         = %[2]q
}

data "glitchtip_team" "test" {
  organization = glitchtip_organization.test.slug
  slug         = glitchtip_team.test.slug
}
`, rName, rTeam),
				Check: r.ComposeAggregateTestCheckFunc(
					r.TestCheckResourceAttrPair("data.glitchtip_team.test", "slug", "glitchtip_team.test", "slug"),
					r.TestCheckResourceAttrPair("data.glitchtip_team.test", "organization", "glitchtip_team.test", "organization"),
					r.TestCheckResourceAttrPair("data.glitchtip_team.test", "id", "glitchtip_team.test", "id"),
					r.TestCheckResourceAttrSet("data.glitchtip_team.test", "date_created"),
					r.TestCheckResourceAttrSet("data.glitchtip_team.test", "member_count"),
				),
			},
		},
	})
}
