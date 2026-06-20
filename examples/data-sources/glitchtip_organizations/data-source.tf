# List all organizations the token can access.
data "glitchtip_organizations" "all" {}

output "organization_slugs" {
  value = [for o in data.glitchtip_organizations.all.organizations : o.slug]
}
