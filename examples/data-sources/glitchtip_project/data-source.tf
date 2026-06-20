data "glitchtip_project" "example" {
  organization = "acme-inc"
  slug         = "api"
}

output "project_platform" {
  value = data.glitchtip_project.example.platform
}
