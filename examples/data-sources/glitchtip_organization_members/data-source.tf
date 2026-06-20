# List all members of an organization.
data "glitchtip_organization_members" "all" {
  organization = "acme-inc"
}

output "member_emails" {
  value = [for m in data.glitchtip_organization_members.all.members : m.email]
}
