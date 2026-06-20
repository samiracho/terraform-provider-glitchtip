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

func TestTeamResourceSchema(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	resp := &fwresource.SchemaResponse{}
	NewTeamResource().Schema(ctx, fwresource.SchemaRequest{}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema method diagnostics: %+v", resp.Diagnostics)
	}
	if diags := resp.Schema.ValidateImplementation(ctx); diags.HasError() {
		t.Fatalf("schema validation diagnostics: %+v", diags)
	}
}

func TestTeamModelFromAPI(t *testing.T) {
	t.Parallel()
	out := teamOut{ID: "42", Slug: "platform", DateCreated: "2026-06-20T00:00:00Z", IsMember: true, MemberCount: 3}
	got := teamModelFromAPI(out, "acme")
	if got.ID.ValueString() != "42" {
		t.Fatalf("ID = %q, want 42", got.ID.ValueString())
	}
	if got.Slug.ValueString() != "platform" {
		t.Fatalf("Slug = %q, want platform", got.Slug.ValueString())
	}
	if got.DateCreated.ValueString() != "2026-06-20T00:00:00Z" {
		t.Fatalf("DateCreated = %q, want 2026-06-20T00:00:00Z", got.DateCreated.ValueString())
	}
	if got.MemberCount.ValueInt64() != 3 {
		t.Fatalf("MemberCount = %d, want 3", got.MemberCount.ValueInt64())
	}
	// organization is not returned by the API and must be carried through.
	if got.Organization.ValueString() != "acme" {
		t.Fatalf("Organization = %q, want acme (carried through)", got.Organization.ValueString())
	}
}

func TestTeamPaths(t *testing.T) {
	t.Parallel()
	if got := teamPath("my org"); got != "/api/0/organizations/my%20org/teams/" {
		t.Fatalf("teamPath escaping = %q", got)
	}
	if got := teamItemPath("my org", "my team"); got != "/api/0/teams/my%20org/my%20team/" {
		t.Fatalf("teamItemPath escaping = %q", got)
	}
}

// --- Acceptance tests (require TF_ACC=1 and a live GlitchTip instance) ---

func TestAccTeamResource_basic(t *testing.T) {
	rOrg := acctest.RandomWithPrefix("tf-acc-org")
	rSlug := acctest.RandomWithPrefix("tf-acc-team")
	rSlugUpdated := rSlug + "-updated"

	r.Test(t, r.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckTeamDestroy,
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
`, rOrg, rSlug),
				Check: r.ComposeAggregateTestCheckFunc(
					r.TestCheckResourceAttr("glitchtip_team.test", "slug", rSlug),
					r.TestCheckResourceAttrSet("glitchtip_team.test", "organization"),
					r.TestCheckResourceAttrSet("glitchtip_team.test", "id"),
					r.TestCheckResourceAttrSet("glitchtip_team.test", "date_created"),
					r.TestCheckResourceAttrSet("glitchtip_team.test", "member_count"),
				),
			},
			{
				ResourceName:      "glitchtip_team.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: testAccTeamImportID,
			},
			{
				Config: providerConfig + fmt.Sprintf(`
resource "glitchtip_organization" "test" {
  name = %[1]q
}

resource "glitchtip_team" "test" {
  organization = glitchtip_organization.test.slug
  slug         = %[2]q
}
`, rOrg, rSlugUpdated),
				Check: r.TestCheckResourceAttr("glitchtip_team.test", "slug", rSlugUpdated),
			},
		},
	})
}

func testAccTeamImportID(s *terraform.State) (string, error) {
	rs, ok := s.RootModule().Resources["glitchtip_team.test"]
	if !ok {
		return "", fmt.Errorf("team resource not found in state")
	}
	return fmt.Sprintf("%s/%s", rs.Primary.Attributes["organization"], rs.Primary.Attributes["id"]), nil
}

func testAccCheckTeamDestroy(s *terraform.State) error {
	c := testAccClient()
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "glitchtip_team" {
			continue
		}
		err := c.Do(context.Background(), http.MethodGet,
			teamItemPath(rs.Primary.Attributes["organization"], rs.Primary.Attributes["slug"]), nil, nil)
		if client.IsNotFound(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("checking team %s: %w", rs.Primary.Attributes["slug"], err)
		}
		return fmt.Errorf("team %s still exists", rs.Primary.Attributes["slug"])
	}
	return nil
}
