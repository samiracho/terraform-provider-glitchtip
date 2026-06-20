// Copyright (c) samiracho
// SPDX-License-Identifier: MIT

package provider

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	r "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/samiracho/glitchip-terraform-provider/internal/client"
)

// --- Unit tests (no live instance required) ---

func TestProjectAlertResourceSchema(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	resp := &fwresource.SchemaResponse{}
	NewProjectAlertResource().Schema(ctx, fwresource.SchemaRequest{}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema method diagnostics: %+v", resp.Diagnostics)
	}
	if diags := resp.Schema.ValidateImplementation(ctx); diags.HasError() {
		t.Fatalf("schema validation diagnostics: %+v", diags)
	}
}

func projectAlertStrPtr(s string) *string { return &s }

func projectAlertTagsList(vals ...string) types.List {
	elems := make([]attr.Value, len(vals))
	for i, v := range vals {
		elems[i] = types.StringValue(v)
	}
	return types.ListValueMust(types.StringType, elems)
}

func TestExpandAlertRecipients(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   []alertRecipientModel
		want []alertRecipientIn
	}{
		{
			name: "nil yields nil",
			in:   nil,
			want: nil,
		},
		{
			name: "email with empty url and no tags",
			in: []alertRecipientModel{
				{RecipientType: types.StringValue("email"), URL: types.StringValue(""), TagsToAdd: types.ListNull(types.StringType)},
			},
			want: []alertRecipientIn{
				{RecipientType: "email", URL: projectAlertStrPtr(""), TagsToAdd: nil},
			},
		},
		{
			name: "webhook with url and tags, plus null-url discord",
			in: []alertRecipientModel{
				{
					RecipientType: types.StringValue("webhook"),
					URL:           types.StringValue("https://example.com/hook"),
					TagsToAdd:     projectAlertTagsList("env", "server_name"),
				},
				{RecipientType: types.StringValue("discord"), URL: types.StringNull(), TagsToAdd: types.ListNull(types.StringType)},
			},
			want: []alertRecipientIn{
				{RecipientType: "webhook", URL: projectAlertStrPtr("https://example.com/hook"), TagsToAdd: []string{"env", "server_name"}},
				{RecipientType: "discord", URL: nil, TagsToAdd: nil},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := expandAlertRecipients(context.Background(), tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("len(got)=%d want %d (%+v)", len(got), len(tc.want), got)
			}
			for i := range got {
				if got[i].RecipientType != tc.want[i].RecipientType {
					t.Errorf("recipient[%d].RecipientType=%q want %q", i, got[i].RecipientType, tc.want[i].RecipientType)
				}
				if !projectAlertPtrStrEqual(got[i].URL, tc.want[i].URL) {
					t.Errorf("recipient[%d].URL=%v want %v", i, projectAlertDeref(got[i].URL), projectAlertDeref(tc.want[i].URL))
				}
				if len(got[i].TagsToAdd) != len(tc.want[i].TagsToAdd) {
					t.Errorf("recipient[%d].TagsToAdd=%v want %v", i, got[i].TagsToAdd, tc.want[i].TagsToAdd)
					continue
				}
				for j := range got[i].TagsToAdd {
					if got[i].TagsToAdd[j] != tc.want[i].TagsToAdd[j] {
						t.Errorf("recipient[%d].TagsToAdd[%d]=%q want %q", i, j, got[i].TagsToAdd[j], tc.want[i].TagsToAdd[j])
					}
				}
			}
		})
	}
}

func TestFlattenAlertRecipients(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		in        []alertRecipientOut
		wantTypes []string
		wantURLs  []string
		wantTags  [][]string
	}{
		{
			name:      "empty yields nil",
			in:        nil,
			wantTypes: nil,
		},
		{
			name: "email empty url, webhook with url and tags",
			in: []alertRecipientOut{
				{RecipientType: "email", URL: projectAlertStrPtr("")},
				{RecipientType: "webhook", URL: projectAlertStrPtr("https://example.com/hook"), TagsToAdd: []string{"env"}},
			},
			wantTypes: []string{"email", "webhook"},
			wantURLs:  []string{"", "https://example.com/hook"},
			wantTags:  [][]string{nil, {"env"}},
		},
		{
			name: "null url maps to null string",
			in: []alertRecipientOut{
				{RecipientType: "discord", URL: nil},
			},
			wantTypes: []string{"discord"},
			wantURLs:  []string{""},
			wantTags:  [][]string{nil},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := flattenAlertRecipients(context.Background(), tc.in)
			if len(got) != len(tc.wantTypes) {
				t.Fatalf("len(got)=%d want %d", len(got), len(tc.wantTypes))
			}
			for i := range got {
				if got[i].RecipientType.ValueString() != tc.wantTypes[i] {
					t.Errorf("recipient[%d].RecipientType=%q want %q", i, got[i].RecipientType.ValueString(), tc.wantTypes[i])
				}
				if got[i].URL.ValueString() != tc.wantURLs[i] {
					t.Errorf("recipient[%d].URL=%q want %q", i, got[i].URL.ValueString(), tc.wantURLs[i])
				}
				wantTags := tc.wantTags[i]
				if wantTags == nil {
					if !got[i].TagsToAdd.IsNull() {
						t.Errorf("recipient[%d].TagsToAdd expected null, got %v", i, got[i].TagsToAdd)
					}
					continue
				}
				var elems []string
				got[i].TagsToAdd.ElementsAs(context.Background(), &elems, false)
				if len(elems) != len(wantTags) {
					t.Errorf("recipient[%d].TagsToAdd=%v want %v", i, elems, wantTags)
					continue
				}
				for j := range elems {
					if elems[j] != wantTags[j] {
						t.Errorf("recipient[%d].TagsToAdd[%d]=%q want %q", i, j, elems[j], wantTags[j])
					}
				}
			}
		})
	}
}

func TestExpandAlertRecipientsEmptyIsNonNil(t *testing.T) {
	t.Parallel()
	// GlitchTip returns HTTP 500 if alertRecipients is JSON null, so an empty
	// or absent list must still marshal to [] (a non-nil slice).
	for _, in := range [][]alertRecipientModel{nil, {}} {
		got := expandAlertRecipients(context.Background(), in)
		if got == nil {
			t.Fatalf("expandAlertRecipients(%v) = nil; want non-nil empty slice", in)
		}
		if len(got) != 0 {
			t.Fatalf("expandAlertRecipients(%v) len = %d; want 0", in, len(got))
		}
	}
}

func TestProjectAlertModelFromAPIEmptyNameIsNull(t *testing.T) {
	t.Parallel()
	id := int64(1)
	empty := ""
	out := projectAlertOut{ID: &id, Name: &empty}
	got := projectAlertModelFromAPI(context.Background(), out, types.StringValue("org"), types.StringValue("proj"))
	if !got.Name.IsNull() {
		t.Fatalf("empty API name should map to null, got %q", got.Name.ValueString())
	}
}

func TestProjectAlertPath(t *testing.T) {
	t.Parallel()
	if got := projectAlertPath("my org", "my proj"); got != "/api/0/projects/my%20org/my%20proj/alerts/" {
		t.Fatalf("projectAlertPath escaping = %q", got)
	}
	if got := projectAlertItemPath("my org", "my proj", 42); got != "/api/0/projects/my%20org/my%20proj/alerts/42/" {
		t.Fatalf("projectAlertItemPath = %q", got)
	}
}

// --- Acceptance tests (require TF_ACC=1 and a live GlitchTip instance) ---

func TestAccProjectAlertResource_basic(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-alert")

	r.Test(t, r.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckProjectAlertDestroy,
		Steps: []r.TestStep{
			{
				Config: testAccProjectAlertConfig(rName, 10),
				Check: r.ComposeAggregateTestCheckFunc(
					r.TestCheckResourceAttr("glitchtip_project_alert.test", "name", rName),
					r.TestCheckResourceAttr("glitchtip_project_alert.test", "timespan_minutes", "60"),
					r.TestCheckResourceAttr("glitchtip_project_alert.test", "quantity", "10"),
					r.TestCheckResourceAttr("glitchtip_project_alert.test", "uptime", "false"),
					r.TestCheckResourceAttr("glitchtip_project_alert.test", "alert_recipients.#", "2"),
					r.TestCheckResourceAttr("glitchtip_project_alert.test", "alert_recipients.0.recipient_type", "email"),
					r.TestCheckResourceAttr("glitchtip_project_alert.test", "alert_recipients.1.recipient_type", "webhook"),
					r.TestCheckResourceAttr("glitchtip_project_alert.test", "alert_recipients.1.url", "https://example.com/hook"),
					r.TestCheckResourceAttrSet("glitchtip_project_alert.test", "id"),
				),
			},
			{
				ResourceName:      "glitchtip_project_alert.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: testAccProjectAlertImportID,
			},
			{
				Config: testAccProjectAlertConfig(rName, 25),
				Check: r.ComposeAggregateTestCheckFunc(
					r.TestCheckResourceAttr("glitchtip_project_alert.test", "quantity", "25"),
				),
			},
		},
	})
}

func testAccProjectAlertConfig(rName string, quantity int) string {
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
  platform     = "python"
}

resource "glitchtip_project_alert" "test" {
  organization     = glitchtip_organization.test.slug
  project          = glitchtip_project.test.slug
  name             = %[1]q
  timespan_minutes = 60
  quantity         = %[2]d

  alert_recipients = [
    {
      recipient_type = "email"
    },
    {
      recipient_type = "webhook"
      url            = "https://example.com/hook"
    },
  ]
}
`, rName, quantity)
}

func testAccProjectAlertImportID(s *terraform.State) (string, error) {
	rs, ok := s.RootModule().Resources["glitchtip_project_alert.test"]
	if !ok {
		return "", fmt.Errorf("project alert resource not found in state")
	}
	return fmt.Sprintf("%s/%s/%s",
		rs.Primary.Attributes["organization"],
		rs.Primary.Attributes["project"],
		rs.Primary.Attributes["id"],
	), nil
}

// TestAccProjectAlertResource_noRecipients covers an alert with no recipients
// and no name. GlitchTip returns HTTP 500 when alertRecipients is null, and the
// API echoes name "" (which must collapse to null); both paths are untested by
// the basic test.
func TestAccProjectAlertResource_noRecipients(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-alert")

	r.Test(t, r.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckProjectAlertDestroy,
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

resource "glitchtip_project_alert" "test" {
  organization     = glitchtip_organization.test.slug
  project          = glitchtip_project.test.slug
  timespan_minutes = 10
  quantity         = 5
}
`, rName),
				Check: r.ComposeAggregateTestCheckFunc(
					r.TestCheckResourceAttrSet("glitchtip_project_alert.test", "id"),
					r.TestCheckNoResourceAttr("glitchtip_project_alert.test", "name"),
				),
			},
		},
	})
}

func testAccCheckProjectAlertDestroy(s *terraform.State) error {
	c := testAccClient()
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "glitchtip_project_alert" {
			continue
		}
		org := rs.Primary.Attributes["organization"]
		project := rs.Primary.Attributes["project"]
		wantID := rs.Primary.Attributes["id"]

		var out []projectAlertOut
		err := c.Do(context.Background(), http.MethodGet, projectAlertPath(org, project), nil, &out)
		if client.IsNotFound(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("listing alerts for %s/%s: %w", org, project, err)
		}
		for _, a := range out {
			if a.ID != nil && fmt.Sprintf("%d", *a.ID) == wantID {
				return fmt.Errorf("project alert %s still exists in %s/%s", wantID, org, project)
			}
		}
	}
	return nil
}

// --- small helpers used only by the unit tests above ---

func projectAlertPtrStrEqual(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func projectAlertDeref(p *string) string {
	if p == nil {
		return "<nil>"
	}
	return *p
}
