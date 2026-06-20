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

func TestProjectDataSourceSchema(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	resp := &fwdatasource.SchemaResponse{}
	NewProjectDataSource().Schema(ctx, fwdatasource.SchemaRequest{}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema method diagnostics: %+v", resp.Diagnostics)
	}
	if diags := resp.Schema.ValidateImplementation(ctx); diags.HasError() {
		t.Fatalf("schema validation diagnostics: %+v", diags)
	}
}

func TestProjectDataSourceModelFromAPI(t *testing.T) {
	t.Parallel()
	platform := "python"
	out := projectDataSourceOut{
		ID:                "12",
		Slug:              "backend",
		Name:              "Backend",
		Platform:          &platform,
		ScrubIPAddresses:  true,
		DateCreated:       "2026-06-20T00:00:00Z",
		EventThrottleRate: 25,
	}
	got := projectDataSourceModelFromAPI(out, "acme")
	if got.ID.ValueString() != "12" || got.Slug.ValueString() != "backend" ||
		got.Name.ValueString() != "Backend" || got.DateCreated.ValueString() != "2026-06-20T00:00:00Z" {
		t.Fatalf("unexpected model: %+v", got)
	}
	if got.Organization.ValueString() != "acme" {
		t.Fatalf("organization not carried through: %+v", got)
	}
	if got.Platform.ValueString() != "python" {
		t.Fatalf("unexpected platform mapping: %+v", got)
	}
	if got.ScrubIPAddresses.ValueBool() != true {
		t.Fatalf("unexpected scrub_ip_addresses mapping: %+v", got)
	}
	if got.EventThrottleRate.ValueInt64() != 25 {
		t.Fatalf("unexpected event_throttle_rate mapping: %+v", got)
	}
}

func TestProjectDataSourceModelFromAPINullPlatform(t *testing.T) {
	t.Parallel()
	out := projectDataSourceOut{
		ID:               "13",
		Slug:             "frontend",
		Name:             "Frontend",
		Platform:         nil,
		ScrubIPAddresses: false,
		DateCreated:      "2026-06-20T00:00:00Z",
	}
	got := projectDataSourceModelFromAPI(out, "acme")
	if !got.Platform.IsNull() {
		t.Fatalf("expected null platform, got: %+v", got.Platform)
	}
}

func TestProjectDataSourcePath(t *testing.T) {
	t.Parallel()
	if got := projectDataSourcePath("my org", "my project"); got != "/api/0/projects/my%20org/my%20project/" {
		t.Fatalf("projectDataSourcePath escaping = %q", got)
	}
}

// --- Acceptance tests (require TF_ACC=1 and a live GlitchTip instance) ---

func TestAccProjectDataSource_basic(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-proj")

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
  slug         = %[1]q
}

resource "glitchtip_project" "test" {
  organization = glitchtip_organization.test.slug
  team         = glitchtip_team.test.slug
  name         = %[1]q
}

data "glitchtip_project" "test" {
  organization = glitchtip_organization.test.slug
  slug         = glitchtip_project.test.slug
}
`, rName),
				Check: r.ComposeAggregateTestCheckFunc(
					r.TestCheckResourceAttr("data.glitchtip_project.test", "name", rName),
					r.TestCheckResourceAttrPair("data.glitchtip_project.test", "slug", "glitchtip_project.test", "slug"),
					r.TestCheckResourceAttrPair("data.glitchtip_project.test", "id", "glitchtip_project.test", "id"),
					r.TestCheckResourceAttrPair("data.glitchtip_project.test", "organization", "glitchtip_organization.test", "slug"),
					r.TestCheckResourceAttrSet("data.glitchtip_project.test", "date_created"),
					r.TestCheckResourceAttrSet("data.glitchtip_project.test", "scrub_ip_addresses"),
					r.TestCheckResourceAttrSet("data.glitchtip_project.test", "event_throttle_rate"),
				),
			},
		},
	})
}
