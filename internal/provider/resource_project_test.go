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

func TestProjectResourceSchema(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	resp := &fwresource.SchemaResponse{}
	NewProjectResource().Schema(ctx, fwresource.SchemaRequest{}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema method diagnostics: %+v", resp.Diagnostics)
	}
	if diags := resp.Schema.ValidateImplementation(ctx); diags.HasError() {
		t.Fatalf("schema validation diagnostics: %+v", diags)
	}
}

func TestProjectModelFromAPI(t *testing.T) {
	t.Parallel()

	platform := "python"
	cases := []struct {
		name string
		out  projectOut
	}{
		{
			name: "with platform",
			out: projectOut{
				ID:                "12",
				Slug:              "my-project",
				Name:              "My Project",
				Platform:          &platform,
				ScrubIPAddresses:  true,
				DateCreated:       "2026-06-20T00:00:00Z",
				EventThrottleRate: 25,
			},
		},
		{
			name: "null platform",
			out: projectOut{
				ID:                "13",
				Slug:              "no-platform",
				Name:              "No Platform",
				Platform:          nil,
				ScrubIPAddresses:  false,
				DateCreated:       "2026-06-20T01:00:00Z",
				EventThrottleRate: 0,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := projectModelFromAPI(tc.out, "acme", "backend-team")

			// Passthrough parent attributes the API does not return.
			if got.Organization.ValueString() != "acme" {
				t.Fatalf("organization = %q, want %q", got.Organization.ValueString(), "acme")
			}
			if got.Team.ValueString() != "backend-team" {
				t.Fatalf("team = %q, want %q", got.Team.ValueString(), "backend-team")
			}
			if got.ID.ValueString() != tc.out.ID {
				t.Fatalf("id = %q, want %q", got.ID.ValueString(), tc.out.ID)
			}
			if got.Slug.ValueString() != tc.out.Slug {
				t.Fatalf("slug = %q, want %q", got.Slug.ValueString(), tc.out.Slug)
			}
			if got.Name.ValueString() != tc.out.Name {
				t.Fatalf("name = %q, want %q", got.Name.ValueString(), tc.out.Name)
			}
			if got.ScrubIPAddresses.ValueBool() != tc.out.ScrubIPAddresses {
				t.Fatalf("scrub_ip_addresses = %v, want %v", got.ScrubIPAddresses.ValueBool(), tc.out.ScrubIPAddresses)
			}
			if got.DateCreated.ValueString() != tc.out.DateCreated {
				t.Fatalf("date_created = %q, want %q", got.DateCreated.ValueString(), tc.out.DateCreated)
			}
			if got.EventThrottleRate.ValueInt64() != tc.out.EventThrottleRate {
				t.Fatalf("event_throttle_rate = %d, want %d", got.EventThrottleRate.ValueInt64(), tc.out.EventThrottleRate)
			}
			if tc.out.Platform == nil {
				if !got.Platform.IsNull() {
					t.Fatalf("platform = %q, want null", got.Platform.ValueString())
				}
			} else if got.Platform.ValueString() != *tc.out.Platform {
				t.Fatalf("platform = %q, want %q", got.Platform.ValueString(), *tc.out.Platform)
			}
		})
	}
}

func TestProjectPaths(t *testing.T) {
	t.Parallel()
	if got := projectPath("my org", "my team"); got != "/api/0/teams/my%20org/my%20team/projects/" {
		t.Fatalf("projectPath escaping = %q", got)
	}
	if got := projectItemPath("my org", "my project"); got != "/api/0/projects/my%20org/my%20project/" {
		t.Fatalf("projectItemPath escaping = %q", got)
	}
}

// --- Acceptance tests (require TF_ACC=1 and a live GlitchTip instance) ---

func TestAccProjectResource_basic(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-proj")

	r.Test(t, r.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckProjectDestroy,
		Steps: []r.TestStep{
			{
				Config: testAccProjectConfig(rName, "python"),
				Check: r.ComposeAggregateTestCheckFunc(
					r.TestCheckResourceAttr("glitchtip_project.test", "name", rName),
					r.TestCheckResourceAttr("glitchtip_project.test", "platform", "python"),
					r.TestCheckResourceAttrSet("glitchtip_project.test", "slug"),
					r.TestCheckResourceAttrSet("glitchtip_project.test", "id"),
					r.TestCheckResourceAttrSet("glitchtip_project.test", "date_created"),
					r.TestCheckResourceAttrSet("glitchtip_project.test", "scrub_ip_addresses"),
					r.TestCheckResourceAttr("glitchtip_project.test", "event_throttle_rate", "0"),
					r.TestCheckResourceAttrPair("glitchtip_project.test", "organization",
						"glitchtip_organization.test", "slug"),
				),
			},
			{
				ResourceName:            "glitchtip_project.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateIdFunc:       testAccProjectImportID,
				ImportStateVerifyIgnore: []string{"team"},
			},
			{
				Config: testAccProjectConfig(rName, "node"),
				Check: r.ComposeAggregateTestCheckFunc(
					r.TestCheckResourceAttr("glitchtip_project.test", "name", rName),
					r.TestCheckResourceAttr("glitchtip_project.test", "platform", "node"),
				),
			},
		},
	})
}

func testAccProjectConfig(rName, platform string) string {
	return providerConfig + fmt.Sprintf(`
resource "glitchtip_organization" "test" {
  name = %[1]q
}

resource "glitchtip_team" "test" {
  organization = glitchtip_organization.test.slug
  slug         = %[1]q
}

resource "glitchtip_project" "test" {
  organization = glitchtip_organization.test.slug
  team         = glitchtip_team.test.slug
  name         = %[1]q
  platform     = %[2]q
}
`, rName, platform)
}

func testAccProjectImportID(s *terraform.State) (string, error) {
	rs, ok := s.RootModule().Resources["glitchtip_project.test"]
	if !ok {
		return "", fmt.Errorf("project resource not found in state")
	}
	return fmt.Sprintf("%s/%s",
		rs.Primary.Attributes["organization"],
		rs.Primary.Attributes["id"],
	), nil
}

// TestAccProjectResource_customSlug covers an explicit slug that differs from
// the name. GlitchTip ignores slug on create (deriving it from name), so the
// provider applies it with a follow-up update; before that fix this failed with
// "provider produced inconsistent result after apply".
func TestAccProjectResource_customSlug(t *testing.T) {
	rOrg := acctest.RandomWithPrefix("tf-acc-org")
	rTeam := acctest.RandomWithPrefix("tf-acc-team")

	r.Test(t, r.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckProjectDestroy,
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

resource "glitchtip_project" "test" {
  organization = glitchtip_organization.test.slug
  team         = glitchtip_team.test.slug
  name         = "Custom Slug Project"
  slug         = "my-custom-slug"
}
`, rOrg, rTeam),
				Check: r.TestCheckResourceAttr("glitchtip_project.test", "slug", "my-custom-slug"),
			},
		},
	})
}

func testAccCheckProjectDestroy(s *terraform.State) error {
	c := testAccClient()
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "glitchtip_project" {
			continue
		}
		err := c.Do(context.Background(), http.MethodGet,
			projectItemPath(rs.Primary.Attributes["organization"], rs.Primary.Attributes["slug"]), nil, nil)
		if client.IsNotFound(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("checking project %s: %w", rs.Primary.Attributes["slug"], err)
		}
		return fmt.Errorf("project %s still exists", rs.Primary.Attributes["slug"])
	}
	return nil
}
