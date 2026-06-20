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

func TestMonitorResourceSchema(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	resp := &fwresource.SchemaResponse{}
	NewMonitorResource().Schema(ctx, fwresource.SchemaRequest{}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema method diagnostics: %+v", resp.Diagnostics)
	}
	if diags := resp.Schema.ValidateImplementation(ctx); diags.HasError() {
		t.Fatalf("schema validation diagnostics: %+v", diags)
	}
}

func TestMonitorModelFromAPI(t *testing.T) {
	t.Parallel()

	id := int64(42)
	timeout := int64(30)
	expectedStatus := int64(200)
	expectedBody := "OK"
	projectID := "9"
	isUp := true
	rawURL := "https://example.com"

	out := monitorOut{
		ID:                    &id,
		Name:                  "api-check",
		URL:                   &rawURL,
		MonitorType:           "GET",
		Interval:              60,
		Timeout:               &timeout,
		ExpectedStatus:        &expectedStatus,
		ExpectedBody:          &expectedBody,
		ConfirmationThreshold: 1,
		ProjectID:             &projectID,
		IsUp:                  &isUp,
		Created:               "2026-06-20T00:00:00Z",
	}

	// organization and confirmation_threshold are not returned by the API and
	// must be carried through; project_id round-trips from the response.
	got := monitorModelFromAPI(out, "acme", 3)

	if got.Organization.ValueString() != "acme" {
		t.Fatalf("organization carry-through = %q, want %q", got.Organization.ValueString(), "acme")
	}
	if got.ID.ValueInt64() != 42 {
		t.Fatalf("id = %d, want 42", got.ID.ValueInt64())
	}
	if got.Name.ValueString() != "api-check" {
		t.Fatalf("name = %q, want %q", got.Name.ValueString(), "api-check")
	}
	if got.URL.ValueString() != "https://example.com" {
		t.Fatalf("url = %q, want %q", got.URL.ValueString(), "https://example.com")
	}
	if got.MonitorType.ValueString() != "GET" {
		t.Fatalf("monitor_type = %q, want %q", got.MonitorType.ValueString(), "GET")
	}
	if got.Interval.ValueInt64() != 60 {
		t.Fatalf("interval = %d, want 60", got.Interval.ValueInt64())
	}
	if got.Timeout.ValueInt64() != 30 {
		t.Fatalf("timeout = %d, want 30", got.Timeout.ValueInt64())
	}
	if got.ExpectedStatus.ValueInt64() != 200 {
		t.Fatalf("expected_status = %d, want 200", got.ExpectedStatus.ValueInt64())
	}
	if got.ExpectedBody.ValueString() != "OK" {
		t.Fatalf("expected_body = %q, want %q", got.ExpectedBody.ValueString(), "OK")
	}
	if got.ConfirmationThreshold.ValueInt64() != 3 {
		t.Fatalf("confirmation_threshold carry-through = %d, want 3", got.ConfirmationThreshold.ValueInt64())
	}
	if got.ProjectID.ValueString() != "9" {
		t.Fatalf("project_id = %q, want %q", got.ProjectID.ValueString(), "9")
	}
	if !got.IsUp.ValueBool() {
		t.Fatalf("is_up = %v, want true", got.IsUp.ValueBool())
	}
	if got.Created.ValueString() != "2026-06-20T00:00:00Z" {
		t.Fatalf("created = %q, want %q", got.Created.ValueString(), "2026-06-20T00:00:00Z")
	}
}

func TestMonitorModelFromAPI_Nulls(t *testing.T) {
	t.Parallel()

	// All nullable fields omitted: pointers map to null types.
	out := monitorOut{
		Name:                  "heartbeat",
		MonitorType:           "Heartbeat",
		Interval:              60,
		ConfirmationThreshold: 1,
		Created:               "2026-06-20T00:00:00Z",
	}

	got := monitorModelFromAPI(out, "acme", 1)

	if !got.ID.IsNull() {
		t.Fatalf("id should be null, got %+v", got.ID)
	}
	if !got.URL.IsNull() {
		t.Fatalf("url should be null, got %+v", got.URL)
	}
	if !got.Timeout.IsNull() {
		t.Fatalf("timeout should be null, got %+v", got.Timeout)
	}
	if !got.ExpectedStatus.IsNull() {
		t.Fatalf("expected_status should be null, got %+v", got.ExpectedStatus)
	}
	// expected_body has a static "" default and is Computed, so a null API
	// response collapses to "" (not null) to keep plan and state consistent.
	if got.ExpectedBody.IsNull() || got.ExpectedBody.ValueString() != "" {
		t.Fatalf("expected_body should be empty string, got %+v", got.ExpectedBody)
	}
	if !got.ProjectID.IsNull() {
		t.Fatalf("project_id should be null, got %+v", got.ProjectID)
	}
	if !got.IsUp.IsNull() {
		t.Fatalf("is_up should be null, got %+v", got.IsUp)
	}
	if !got.ProjectID.IsNull() {
		t.Fatalf("project_id should be null, got %+v", got.ProjectID)
	}
}

func TestMonitorModelFromAPIEmptyURLIsNull(t *testing.T) {
	t.Parallel()
	// url-less monitor types (e.g. Heartbeat) get url "" echoed by the API; it
	// must collapse to null to match the planned (omitted) value.
	empty := ""
	out := monitorOut{Name: "hb", MonitorType: "Heartbeat", Interval: 60, URL: &empty, Created: "2026-06-20T00:00:00Z"}
	got := monitorModelFromAPI(out, "acme", 1)
	if !got.URL.IsNull() {
		t.Fatalf("empty API url should map to null, got %q", got.URL.ValueString())
	}
}

func TestMonitorPaths(t *testing.T) {
	t.Parallel()
	if got := monitorPath("my org"); got != "/api/0/organizations/my%20org/monitors/" {
		t.Fatalf("monitorPath escaping = %q", got)
	}
	if got := monitorItemPath("my org", "12 3"); got != "/api/0/organizations/my%20org/monitors/12%203/" {
		t.Fatalf("monitorItemPath escaping = %q", got)
	}
}

// --- Acceptance tests (require TF_ACC=1 and a live GlitchTip instance) ---

func TestAccMonitorResource_basic(t *testing.T) {
	rOrg := acctest.RandomWithPrefix("tf-acc-org")
	rName := acctest.RandomWithPrefix("tf-acc-monitor")

	r.Test(t, r.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckMonitorDestroy,
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
  name         = %[2]q
}

resource "glitchtip_monitor" "test" {
  organization    = glitchtip_organization.test.slug
  name            = %[2]q
  monitor_type    = "GET"
  url             = "https://example.com"
  interval        = 60
  expected_status = 200
  project_id      = glitchtip_project.test.id
}
`, rOrg, rName),
				Check: r.ComposeAggregateTestCheckFunc(
					r.TestCheckResourceAttr("glitchtip_monitor.test", "name", rName),
					r.TestCheckResourceAttr("glitchtip_monitor.test", "monitor_type", "GET"),
					r.TestCheckResourceAttr("glitchtip_monitor.test", "url", "https://example.com"),
					r.TestCheckResourceAttr("glitchtip_monitor.test", "interval", "60"),
					r.TestCheckResourceAttr("glitchtip_monitor.test", "expected_status", "200"),
					r.TestCheckResourceAttr("glitchtip_monitor.test", "confirmation_threshold", "1"),
					r.TestCheckResourceAttrSet("glitchtip_monitor.test", "id"),
					r.TestCheckResourceAttrSet("glitchtip_monitor.test", "created"),
					// project_id round-trips: it must equal the attached project's id.
					r.TestCheckResourceAttrPair("glitchtip_monitor.test", "project_id", "glitchtip_project.test", "id"),
				),
			},
			{
				ResourceName:      "glitchtip_monitor.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: testAccMonitorImportID,
				// confirmation_threshold is write-only (the API never returns it),
				// so it cannot be recovered on import; is_up is volatile runtime
				// state that flips as checks run. Both are exempt from verification.
				ImportStateVerifyIgnore: []string{"confirmation_threshold", "is_up"},
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

resource "glitchtip_project" "test" {
  organization = glitchtip_organization.test.slug
  team         = glitchtip_team.test.slug
  name         = %[2]q
}

resource "glitchtip_monitor" "test" {
  organization    = glitchtip_organization.test.slug
  name            = %[2]q
  monitor_type    = "GET"
  url             = "https://example.com"
  interval        = 120
  expected_status = 200
  project_id      = glitchtip_project.test.id
}
`, rOrg, rName),
				Check: r.TestCheckResourceAttr("glitchtip_monitor.test", "interval", "120"),
			},
		},
	})
}

func testAccMonitorImportID(s *terraform.State) (string, error) {
	rs, ok := s.RootModule().Resources["glitchtip_monitor.test"]
	if !ok {
		return "", fmt.Errorf("monitor resource not found in state")
	}
	return fmt.Sprintf("%s/%s",
		rs.Primary.Attributes["organization"],
		rs.Primary.Attributes["id"],
	), nil
}

// TestAccMonitorResource_heartbeat covers a url-less monitor type. Before the
// empty-url-collapse and expected_status-default fixes, applying this failed
// with "provider produced inconsistent result after apply".
func TestAccMonitorResource_heartbeat(t *testing.T) {
	rOrg := acctest.RandomWithPrefix("tf-acc-org")
	rName := acctest.RandomWithPrefix("tf-acc-hb")

	r.Test(t, r.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckMonitorDestroy,
		Steps: []r.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`
resource "glitchtip_organization" "test" {
  name = %[1]q
}

resource "glitchtip_monitor" "test" {
  organization = glitchtip_organization.test.slug
  name         = %[2]q
  monitor_type = "Heartbeat"
  interval     = 60
}
`, rOrg, rName),
				Check: r.ComposeAggregateTestCheckFunc(
					r.TestCheckResourceAttr("glitchtip_monitor.test", "monitor_type", "Heartbeat"),
					// url is omitted for Heartbeat; the API echoes "" which must collapse to null.
					r.TestCheckNoResourceAttr("glitchtip_monitor.test", "url"),
					// expected_status was omitted; the default must be applied.
					r.TestCheckResourceAttr("glitchtip_monitor.test", "expected_status", "200"),
				),
			},
		},
	})
}

func testAccCheckMonitorDestroy(s *terraform.State) error {
	c := testAccClient()
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "glitchtip_monitor" {
			continue
		}
		err := c.Do(context.Background(), http.MethodGet,
			monitorItemPath(rs.Primary.Attributes["organization"], rs.Primary.Attributes["id"]), nil, nil)
		if client.IsNotFound(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("checking monitor %s: %w", rs.Primary.Attributes["id"], err)
		}
		return fmt.Errorf("monitor %s still exists", rs.Primary.Attributes["id"])
	}
	return nil
}
