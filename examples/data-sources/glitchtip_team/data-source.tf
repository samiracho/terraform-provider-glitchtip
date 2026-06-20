data "glitchtip_team" "example" {
  organization = "acme-inc"
  slug         = "backend"
}

output "team_member_count" {
  value = data.glitchtip_team.example.member_count
}
