# Associate an existing project with an additional team. The project is created
# under one team; use this resource to grant further teams access to it.
resource "glitchtip_team" "frontend" {
  organization = glitchtip_organization.example.slug
  slug         = "frontend"
}

resource "glitchtip_project_team" "example" {
  organization = glitchtip_organization.example.slug
  project      = glitchtip_project.example.slug
  team         = glitchtip_team.frontend.slug
}
