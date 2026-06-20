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

func TestProjectsDataSourceSchema(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	resp := &fwdatasource.SchemaResponse{}
	NewProjectsDataSource().Schema(ctx, fwdatasource.SchemaRequest{}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema method diagnostics: %+v", resp.Diagnostics)
	}
	if diags := resp.Schema.ValidateImplementation(ctx); diags.HasError() {
		t.Fatalf("schema validation diagnostics: %+v", diags)
	}
}

func TestProjectsItemFromAPI(t *testing.T) {
	t.Parallel()
	slug := "api"
	platform := "python"
	got := projectsItemFromAPI(projectsListItem{
		ID: "7", Slug: &slug, Name: "API", Platform: &platform,
		DateCreated: "2026-06-20T00:00:00Z", ScrubIPAddresses: true, EventThrottleRate: 25,
	})
	if got.ID.ValueString() != "7" || got.Slug.ValueString() != "api" ||
		got.Name.ValueString() != "API" || got.Platform.ValueString() != "python" ||
		!got.ScrubIPAddresses.ValueBool() || got.EventThrottleRate.ValueInt64() != 25 {
		t.Fatalf("unexpected mapping: %+v", got)
	}

	// Null slug/platform map to null.
	gotNull := projectsItemFromAPI(projectsListItem{ID: "8", Name: "x"})
	if !gotNull.Slug.IsNull() {
		t.Fatalf("nil slug should map to null, got %q", gotNull.Slug.ValueString())
	}
	if !gotNull.Platform.IsNull() {
		t.Fatalf("nil platform should map to null, got %q", gotNull.Platform.ValueString())
	}
}

// --- Acceptance test ---

func TestAccProjectsDataSource_basic(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-projects")

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

resource "glitchtip_project" "a" {
  organization = glitchtip_organization.test.slug
  team         = glitchtip_team.test.slug
  name         = "proj-a"
}

resource "glitchtip_project" "b" {
  organization = glitchtip_organization.test.slug
  team         = glitchtip_team.test.slug
  name         = "proj-b"
}

data "glitchtip_projects" "all" {
  organization = glitchtip_organization.test.slug
  depends_on   = [glitchtip_project.a, glitchtip_project.b]
}
`, rName),
				Check: r.ComposeAggregateTestCheckFunc(
					// The freshly-created org has exactly the two projects.
					r.TestCheckResourceAttr("data.glitchtip_projects.all", "projects.#", "2"),
					r.TestCheckResourceAttrSet("data.glitchtip_projects.all", "projects.0.id"),
					r.TestCheckResourceAttrSet("data.glitchtip_projects.all", "projects.0.slug"),
				),
			},
		},
	})
}
