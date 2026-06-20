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

func TestOrganizationsDataSourceSchema(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	resp := &fwdatasource.SchemaResponse{}
	NewOrganizationsDataSource().Schema(ctx, fwdatasource.SchemaRequest{}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema method diagnostics: %+v", resp.Diagnostics)
	}
	if diags := resp.Schema.ValidateImplementation(ctx); diags.HasError() {
		t.Fatalf("schema validation diagnostics: %+v", diags)
	}
}

func TestOrganizationsItemFromAPI(t *testing.T) {
	t.Parallel()
	slug := "acme"
	name := "Acme"
	dateCreated := "2026-06-20T00:00:00Z"
	accepting := true
	var throttle int64 = 25
	got := organizationsItemFromAPI(organizationsListItem{
		ID: "7", Slug: &slug, Name: &name, DateCreated: &dateCreated,
		IsAcceptingEvents: &accepting, EventThrottleRate: &throttle,
	})
	if got.ID.ValueString() != "7" || got.Slug.ValueString() != "acme" ||
		got.Name.ValueString() != "Acme" || got.DateCreated.ValueString() != "2026-06-20T00:00:00Z" ||
		!got.IsAcceptingEvents.ValueBool() || got.EventThrottleRate.ValueInt64() != 25 {
		t.Fatalf("unexpected mapping: %+v", got)
	}

	// Null pointer fields map to null.
	gotNull := organizationsItemFromAPI(organizationsListItem{ID: "8"})
	if !gotNull.Slug.IsNull() {
		t.Fatalf("nil slug should map to null, got %q", gotNull.Slug.ValueString())
	}
	if !gotNull.Name.IsNull() {
		t.Fatalf("nil name should map to null, got %q", gotNull.Name.ValueString())
	}
	if !gotNull.DateCreated.IsNull() {
		t.Fatalf("nil date_created should map to null, got %q", gotNull.DateCreated.ValueString())
	}
	if !gotNull.IsAcceptingEvents.IsNull() {
		t.Fatalf("nil is_accepting_events should map to null, got %v", gotNull.IsAcceptingEvents.ValueBool())
	}
	if !gotNull.EventThrottleRate.IsNull() {
		t.Fatalf("nil event_throttle_rate should map to null, got %d", gotNull.EventThrottleRate.ValueInt64())
	}
}

// --- Acceptance test ---

func TestAccOrganizationsDataSource_basic(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-orgs")

	r.Test(t, r.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []r.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`
resource "glitchtip_organization" "a" {
  name = "%[1]s-a"
}

resource "glitchtip_organization" "b" {
  name = "%[1]s-b"
}

data "glitchtip_organizations" "all" {
  depends_on = [glitchtip_organization.a, glitchtip_organization.b]
}
`, rName),
				// The instance is shared and may already contain other
				// organizations, so an exact count cannot be asserted; assert
				// instead that the list is populated.
				Check: r.ComposeAggregateTestCheckFunc(
					r.TestCheckResourceAttrSet("data.glitchtip_organizations.all", "organizations.0.id"),
				),
			},
		},
	})
}
