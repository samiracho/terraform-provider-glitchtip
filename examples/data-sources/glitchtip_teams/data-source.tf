# List all teams in an organization.
data "glitchtip_teams" "all" {
  organization = "acme-inc"
}

output "team_slugs" {
  value = [for t in data.glitchtip_teams.all.teams : t.slug]
}
