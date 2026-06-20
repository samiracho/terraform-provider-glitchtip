resource "glitchtip_organization" "example" {
  name = "Acme Inc"
}

resource "glitchtip_team" "example" {
  organization = glitchtip_organization.example.slug
  slug         = "backend"
}
