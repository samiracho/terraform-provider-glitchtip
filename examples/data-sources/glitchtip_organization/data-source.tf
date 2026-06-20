data "glitchtip_organization" "example" {
  slug = "acme-inc"
}

output "organization_id" {
  value = data.glitchtip_organization.example.id
}
