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

func TestOrganizationDataSourceSchema(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	resp := &fwdatasource.SchemaResponse{}
	NewOrganizationDataSource().Schema(ctx, fwdatasource.SchemaRequest{}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema method diagnostics: %+v", resp.Diagnostics)
	}
	if diags := resp.Schema.ValidateImplementation(ctx); diags.HasError() {
		t.Fatalf("schema validation diagnostics: %+v", diags)
	}
}

func TestOrganizationDataSourceModelFromAPI(t *testing.T) {
	t.Parallel()
	out := organizationDataSourceOut{
		ID:                "7",
		Slug:              "acme",
		Name:              "Acme",
		DateCreated:       "2026-06-20T00:00:00Z",
		IsAcceptingEvents: true,
		OpenMembership:    false,
	}
	got := organizationDataSourceModelFromAPI(out)
	if got.ID.ValueString() != "7" || got.Slug.ValueString() != "acme" ||
		got.Name.ValueString() != "Acme" || got.DateCreated.ValueString() != "2026-06-20T00:00:00Z" {
		t.Fatalf("unexpected model: %+v", got)
	}
	if got.IsAcceptingEvents.ValueBool() != true || got.OpenMembership.ValueBool() != false {
		t.Fatalf("unexpected bool mapping: %+v", got)
	}
}

func TestOrganizationDataSourcePath(t *testing.T) {
	t.Parallel()
	if got := organizationDataSourcePath("my org"); got != "/api/0/organizations/my%20org/" {
		t.Fatalf("organizationDataSourcePath escaping = %q", got)
	}
}

// --- Acceptance tests (require TF_ACC=1 and a live GlitchTip instance) ---

func TestAccOrganizationDataSource_basic(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-org")

	r.Test(t, r.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []r.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`
resource "glitchtip_organization" "test" {
  name = %[1]q
}

data "glitchtip_organization" "test" {
  slug = glitchtip_organization.test.slug
}
`, rName),
				Check: r.ComposeAggregateTestCheckFunc(
					r.TestCheckResourceAttr("data.glitchtip_organization.test", "name", rName),
					r.TestCheckResourceAttrPair("data.glitchtip_organization.test", "slug", "glitchtip_organization.test", "slug"),
					r.TestCheckResourceAttrPair("data.glitchtip_organization.test", "id", "glitchtip_organization.test", "id"),
					r.TestCheckResourceAttrSet("data.glitchtip_organization.test", "date_created"),
					r.TestCheckResourceAttrSet("data.glitchtip_organization.test", "is_accepting_events"),
					r.TestCheckResourceAttrSet("data.glitchtip_organization.test", "open_membership"),
				),
			},
		},
	})
}
