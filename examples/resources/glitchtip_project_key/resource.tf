resource "glitchtip_project_key" "example" {
  organization = glitchtip_organization.example.slug
  project      = glitchtip_project.example.slug
  name         = "production"

  rate_limit = {
    window = 60
    count  = 1000
  }
}

output "dsn" {
  value     = glitchtip_project_key.example.dsn
  sensitive = true
}
