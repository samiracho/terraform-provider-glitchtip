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

func TestMonitorsDataSourceSchema(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	resp := &fwdatasource.SchemaResponse{}
	NewMonitorsDataSource().Schema(ctx, fwdatasource.SchemaRequest{}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema method diagnostics: %+v", resp.Diagnostics)
	}
	if diags := resp.Schema.ValidateImplementation(ctx); diags.HasError() {
		t.Fatalf("schema validation diagnostics: %+v", diags)
	}
}

func TestMonitorsItemFromAPI(t *testing.T) {
	t.Parallel()
	id := int64(42)
	rawURL := "https://example.com"
	isUp := true
	projectID := "9"
	got := monitorsItemFromAPI(monitorsListItem{
		ID: &id, Name: "api-check", MonitorType: "GET", URL: &rawURL,
		Interval: 60, IsUp: &isUp, ProjectID: &projectID, Created: "2026-06-20T00:00:00Z",
	})
	if got.ID.ValueInt64() != 42 || got.Name.ValueString() != "api-check" ||
		got.MonitorType.ValueString() != "GET" || got.URL.ValueString() != "https://example.com" ||
		got.Interval.ValueInt64() != 60 || !got.IsUp.ValueBool() ||
		got.ProjectID.ValueString() != "9" || got.Created.ValueString() != "2026-06-20T00:00:00Z" {
		t.Fatalf("unexpected mapping: %+v", got)
	}

	// Null id/url/is_up/project_id map to null.
	gotNull := monitorsItemFromAPI(monitorsListItem{Name: "heartbeat", MonitorType: "Heartbeat", Interval: 60})
	if !gotNull.ID.IsNull() {
		t.Fatalf("nil id should map to null, got %d", gotNull.ID.ValueInt64())
	}
	if !gotNull.URL.IsNull() {
		t.Fatalf("nil url should map to null, got %q", gotNull.URL.ValueString())
	}
	if !gotNull.IsUp.IsNull() {
		t.Fatalf("nil is_up should map to null, got %v", gotNull.IsUp.ValueBool())
	}
	if !gotNull.ProjectID.IsNull() {
		t.Fatalf("nil project_id should map to null, got %q", gotNull.ProjectID.ValueString())
	}
}

// --- Acceptance test ---

func TestAccMonitorsDataSource_basic(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-monitors")

	r.Test(t, r.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []r.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`
resource "glitchtip_organization" "test" {
  name = %[1]q
}

resource "glitchtip_monitor" "test" {
  organization    = glitchtip_organization.test.slug
  name            = %[1]q
  monitor_type    = "GET"
  url             = "https://example.com"
  interval        = 60
  expected_status = 200
}

data "glitchtip_monitors" "all" {
  organization = glitchtip_organization.test.slug
  depends_on   = [glitchtip_monitor.test]
}
`, rName),
				Check: r.ComposeAggregateTestCheckFunc(
					// The freshly-created org has exactly the one monitor.
					r.TestCheckResourceAttr("data.glitchtip_monitors.all", "monitors.#", "1"),
					r.TestCheckResourceAttr("data.glitchtip_monitors.all", "monitors.0.monitor_type", "GET"),
					r.TestCheckResourceAttrSet("data.glitchtip_monitors.all", "monitors.0.id"),
				),
			},
		},
	})
}
