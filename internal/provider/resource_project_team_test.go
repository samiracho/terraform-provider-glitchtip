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

	"github.com/samiracho/glitchip-terraform-provider/internal/client"
)

// --- Unit tests (no live instance required) ---

func TestProjectTeamResourceSchema(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	resp := &fwresource.SchemaResponse{}
	NewProjectTeamResource().Schema(ctx, fwresource.SchemaRequest{}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema method diagnostics: %+v", resp.Diagnostics)
	}
	if diags := resp.Schema.ValidateImplementation(ctx); diags.HasError() {
		t.Fatalf("schema validation diagnostics: %+v", diags)
	}
}

func TestProjectTeamModelFromAPI(t *testing.T) {
	t.Parallel()
	got := projectTeamModelFromAPI("acme", "backend", "extra")
	if got.ID.ValueString() != "acme/backend/extra" {
		t.Fatalf("unexpected id: %q", got.ID.ValueString())
	}
	if got.Organization.ValueString() != "acme" ||
		got.Project.ValueString() != "backend" ||
		got.Team.ValueString() != "extra" {
		t.Fatalf("unexpected model: %+v", got)
	}
}

func TestProjectTeamPaths(t *testing.T) {
	t.Parallel()
	if got := projectTeamItemPath("acme", "back end", "ex tra"); got != "/api/0/projects/acme/back%20end/teams/ex%20tra/" {
		t.Fatalf("projectTeamItemPath escaping = %q", got)
	}
	if got := projectTeamListPath("acme", "back end"); got != "/api/0/projects/acme/back%20end/teams/" {
		t.Fatalf("projectTeamListPath escaping = %q", got)
	}
}

// --- Acceptance tests (require TF_ACC=1 and a live GlitchTip instance) ---

func TestAccProjectTeamResource_basic(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-pt")

	r.Test(t, r.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckProjectTeamDestroy,
		Steps: []r.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`
resource "glitchtip_organization" "test" {
  name = %[1]q
}

resource "glitchtip_team" "owner" {
  organization = glitchtip_organization.test.slug
  slug         = "%[1]s-owner"
}

resource "glitchtip_team" "extra" {
  organization = glitchtip_organization.test.slug
  slug         = "%[1]s-extra"
}

resource "glitchtip_project" "test" {
  organization = glitchtip_organization.test.slug
  team         = glitchtip_team.owner.slug
  name         = %[1]q
}

resource "glitchtip_project_team" "test" {
  organization = glitchtip_organization.test.slug
  project      = glitchtip_project.test.slug
  team         = glitchtip_team.extra.slug
}
`, rName),
				Check: r.ComposeAggregateTestCheckFunc(
					r.TestCheckResourceAttrPair("glitchtip_project_team.test", "organization", "glitchtip_organization.test", "slug"),
					r.TestCheckResourceAttrPair("glitchtip_project_team.test", "project", "glitchtip_project.test", "slug"),
					r.TestCheckResourceAttrPair("glitchtip_project_team.test", "team", "glitchtip_team.extra", "slug"),
					r.TestCheckResourceAttrSet("glitchtip_project_team.test", "id"),
				),
			},
			{
				ResourceName:      "glitchtip_project_team.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: testAccProjectTeamImportID,
			},
		},
	})
}

func testAccProjectTeamImportID(s *terraform.State) (string, error) {
	rs, ok := s.RootModule().Resources["glitchtip_project_team.test"]
	if !ok {
		return "", fmt.Errorf("project_team resource not found in state")
	}
	return fmt.Sprintf("%s/%s/%s",
		rs.Primary.Attributes["organization"],
		rs.Primary.Attributes["project"],
		rs.Primary.Attributes["team"]), nil
}

func testAccCheckProjectTeamDestroy(s *terraform.State) error {
	c := testAccClient()
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "glitchtip_project_team" {
			continue
		}
		org := rs.Primary.Attributes["organization"]
		project := rs.Primary.Attributes["project"]
		team := rs.Primary.Attributes["team"]

		var out []projectTeamOut
		err := c.Do(context.Background(), http.MethodGet,
			projectTeamListPath(org, project), nil, &out)
		if client.IsNotFound(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("listing teams for project %s/%s: %w", org, project, err)
		}
		for _, t := range out {
			if t.Slug == team {
				return fmt.Errorf("project_team %s/%s/%s still exists", org, project, team)
			}
		}
	}
	return nil
}
