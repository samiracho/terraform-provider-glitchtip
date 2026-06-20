resource "glitchtip_organization" "example" {
  name = "Acme Inc"
}

resource "glitchtip_team" "example" {
  organization = glitchtip_organization.example.slug
  slug         = "backend"
}

resource "glitchtip_project" "example" {
  organization = glitchtip_organization.example.slug
  team         = glitchtip_team.example.slug
  name         = "api"
  platform     = "python"
}
