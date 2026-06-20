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

	"github.com/samiracho/terraform-provider-glitchtip/internal/client"
)

// --- Unit tests (no live instance required) ---

func TestProjectKeyResourceSchema(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	resp := &fwresource.SchemaResponse{}
	NewProjectKeyResource().Schema(ctx, fwresource.SchemaRequest{}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema method diagnostics: %+v", resp.Diagnostics)
	}
	if diags := resp.Schema.ValidateImplementation(ctx); diags.HasError() {
		t.Fatalf("schema validation diagnostics: %+v", diags)
	}
}

func TestProjectKeyModelFromAPI(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("with rate limit", func(t *testing.T) {
		name := "ingest"
		out := projectKeyOut{
			Name:        &name,
			RateLimit:   &projectKeyRateLimit{Window: 60, Count: 100},
			DateCreated: "2026-06-20T00:00:00Z",
			ID:          "11111111-1111-1111-1111-111111111111",
			DSN:         map[string]string{"public": "https://abc@example.com/1", "secret": "https://abc:def@example.com/1"},
			Public:      "abc",
			ProjectID:   42,
		}
		got, diags := projectKeyModelFromAPI(ctx, out, "acme", "backend")
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %+v", diags)
		}
		if got.Organization.ValueString() != "acme" {
			t.Fatalf("organization carried through wrong: %q", got.Organization.ValueString())
		}
		if got.Project.ValueString() != "backend" {
			t.Fatalf("project carried through wrong: %q", got.Project.ValueString())
		}
		if got.Name.ValueString() != "ingest" {
			t.Fatalf("name = %q", got.Name.ValueString())
		}
		if got.ID.ValueString() != "11111111-1111-1111-1111-111111111111" {
			t.Fatalf("id = %q", got.ID.ValueString())
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
		if got.RateLimit == nil {
			t.Fatalf("rate_limit should be set")
		}
		if got.RateLimit.Window.ValueInt64() != 60 || got.RateLimit.Count.ValueInt64() != 100 {
			t.Fatalf("rate_limit = %+v", got.RateLimit)
		}
		elems := got.DSN.Elements()
		if len(elems) != 2 {
			t.Fatalf("dsn elements = %d", len(elems))
		}
	})

	t.Run("without rate limit and null name", func(t *testing.T) {
		out := projectKeyOut{
			Name:        nil,
			RateLimit:   nil,
			DateCreated: "2026-06-20T00:00:00Z",
			ID:          "22222222-2222-2222-2222-222222222222",
			DSN:         map[string]string{},
			Public:      "def",
			ProjectID:   7,
		}
		got, diags := projectKeyModelFromAPI(ctx, out, "acme", "frontend")
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %+v", diags)
		}
		if !got.Name.IsNull() {
			t.Fatalf("name should be null, got %q", got.Name.ValueString())
		}
		if got.RateLimit != nil {
			t.Fatalf("rate_limit should be nil, got %+v", got.RateLimit)
		}
		if got.Project.ValueString() != "frontend" {
			t.Fatalf("project carried through wrong: %q", got.Project.ValueString())
		}
	})
}

func TestProjectKeyItemPath(t *testing.T) {
	t.Parallel()
	got := projectKeyItemPath("my org", "my proj", "key id")
	want := "/api/0/projects/my%20org/my%20proj/keys/key%20id/"
	if got != want {
		t.Fatalf("projectKeyItemPath escaping = %q, want %q", got, want)
	}
	gotList := projectKeyPath("my org", "my proj")
	wantList := "/api/0/projects/my%20org/my%20proj/keys/"
	if gotList != wantList {
		t.Fatalf("projectKeyPath escaping = %q, want %q", gotList, wantList)
	}
}

// --- Acceptance tests (require TF_ACC=1 and a live GlitchTip instance) ---

func TestAccProjectKeyResource_basic(t *testing.T) {
	rOrg := acctest.RandomWithPrefix("tf-acc-org")
	rTeam := acctest.RandomWithPrefix("tf-acc-team")
	rProject := acctest.RandomWithPrefix("tf-acc-proj")
	rName := acctest.RandomWithPrefix("tf-acc-key")
	rNameUpdated := rName + "-updated"

	r.Test(t, r.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckProjectKeyDestroy,
		Steps: []r.TestStep{
			{
				Config: testAccProjectKeyConfig(rOrg, rTeam, rProject, rName),
				Check: r.ComposeAggregateTestCheckFunc(
					r.TestCheckResourceAttr("glitchtip_project_key.test", "name", rName),
					r.TestCheckResourceAttrSet("glitchtip_project_key.test", "organization"),
					r.TestCheckResourceAttrSet("glitchtip_project_key.test", "project"),
					r.TestCheckResourceAttrSet("glitchtip_project_key.test", "id"),
					r.TestCheckResourceAttrSet("glitchtip_project_key.test", "public"),
					r.TestCheckResourceAttrSet("glitchtip_project_key.test", "project_id"),
					r.TestCheckResourceAttrSet("glitchtip_project_key.test", "date_created"),
					r.TestCheckResourceAttrSet("glitchtip_project_key.test", "dsn.public"),
				),
			},
			{
				ResourceName:      "glitchtip_project_key.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: testAccProjectKeyImportID,
			},
			{
				Config: testAccProjectKeyConfig(rOrg, rTeam, rProject, rNameUpdated),
				Check:  r.TestCheckResourceAttr("glitchtip_project_key.test", "name", rNameUpdated),
			},
		},
	})
}

func testAccProjectKeyConfig(orgName, teamSlug, projectName, keyName string) string {
	return providerConfig + fmt.Sprintf(`
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
  name         = %[3]q
}

resource "glitchtip_project_key" "test" {
  organization = glitchtip_organization.test.slug
  project      = glitchtip_project.test.slug
  name         = %[4]q
}
`, orgName, teamSlug, projectName, keyName)
}

func testAccProjectKeyImportID(s *terraform.State) (string, error) {
	rs, ok := s.RootModule().Resources["glitchtip_project_key.test"]
	if !ok {
		return "", fmt.Errorf("project key resource not found in state")
	}
	return fmt.Sprintf("%s/%s/%s",
		rs.Primary.Attributes["organization"],
		rs.Primary.Attributes["project"],
		rs.Primary.Attributes["id"],
	), nil
}

func testAccCheckProjectKeyDestroy(s *terraform.State) error {
	c := testAccClient()
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "glitchtip_project_key" {
			continue
		}
		err := c.Do(context.Background(), http.MethodGet,
			projectKeyItemPath(
				rs.Primary.Attributes["organization"],
				rs.Primary.Attributes["project"],
				rs.Primary.Attributes["id"],
			), nil, nil)
		if client.IsNotFound(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("checking project key %s: %w", rs.Primary.Attributes["id"], err)
		}
		return fmt.Errorf("project key %s still exists", rs.Primary.Attributes["id"])
	}
	return nil
}
