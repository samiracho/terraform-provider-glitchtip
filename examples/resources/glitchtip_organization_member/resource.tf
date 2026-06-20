resource "glitchtip_organization_member" "example" {
  organization = glitchtip_organization.example.slug
  email        = "developer@example.com"
  org_role     = "member"
}
