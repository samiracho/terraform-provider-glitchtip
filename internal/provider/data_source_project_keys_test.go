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

func TestProjectKeysDataSourceSchema(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	resp := &fwdatasource.SchemaResponse{}
	NewProjectKeysDataSource().Schema(ctx, fwdatasource.SchemaRequest{}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema method diagnostics: %+v", resp.Diagnostics)
	}
	if diags := resp.Schema.ValidateImplementation(ctx); diags.HasError() {
		t.Fatalf("schema validation diagnostics: %+v", diags)
	}
}

func TestProjectKeysItemFromAPI(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	name := "ingest"
	got, diags := projectKeysItemFromAPI(ctx, projectKeysListItem{
		ID:          "11111111-1111-1111-1111-111111111111",
		Name:        &name,
		Public:      "abc",
		ProjectID:   42,
		DSN:         map[string]string{"public": "https://abc@example.com/1", "secret": "https://abc:def@example.com/1"},
		DateCreated: "2026-06-20T00:00:00Z",
	})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}
	if got.ID.ValueString() != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("id = %q", got.ID.ValueString())
	}
	if got.Name.ValueString() != "ingest" {
		t.Fatalf("name = %q", got.Name.ValueString())
	}
	if got.Public.ValueString() != "abc" {
		t.Fatalf("public = %q", got.Public.ValueString())
	}
	if got.ProjectID.ValueInt64() != 42 {
		t.Fatalf("project_id = %d", got.ProjectID.ValueInt64())
	}
	if got.DateCreated.ValueString() != "2026-06-20T00:00:00Z" {
		t.Fatalf("date_created = %q", got.DateCreated.ValueString())
	}
	if elems := got.DSN.Elements(); len(elems) != 2 {
		t.Fatalf("dsn elements = %d", len(elems))
	}

	// Null name maps to null.
	gotNull, diags := projectKeysItemFromAPI(ctx, projectKeysListItem{
		ID:          "22222222-2222-2222-2222-222222222222",
		Name:        nil,
		Public:      "def",
		ProjectID:   7,
		DSN:         map[string]string{},
		DateCreated: "2026-06-20T00:00:00Z",
	})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}
	if !gotNull.Name.IsNull() {
		t.Fatalf("nil name should map to null, got %q", gotNull.Name.ValueString())
	}
}

// --- Acceptance test ---

func TestAccProjectKeysDataSource_basic(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-project-keys")

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
  name         = "proj"
}

resource "glitchtip_project_key" "test" {
  organization = glitchtip_organization.test.slug
  project      = glitchtip_project.test.slug
  name         = "key"
}

data "glitchtip_project_keys" "all" {
  organization = glitchtip_organization.test.slug
  project      = glitchtip_project.test.slug
  depends_on   = [glitchtip_project_key.test]
}
`, rName),
				Check: r.ComposeAggregateTestCheckFunc(
					// GlitchTip auto-creates a default key when the project is
					// created, so the project has two keys: that default plus the
					// one created here.
					r.TestCheckResourceAttr("data.glitchtip_project_keys.all", "keys.#", "2"),
					r.TestCheckResourceAttrSet("data.glitchtip_project_keys.all", "keys.0.id"),
					r.TestCheckResourceAttrSet("data.glitchtip_project_keys.all", "keys.0.public"),
					r.TestCheckResourceAttrSet("data.glitchtip_project_keys.all", "keys.0.dsn.public"),
				),
			},
		},
	})
}
