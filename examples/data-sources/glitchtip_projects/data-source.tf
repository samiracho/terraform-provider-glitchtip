# List all projects in an organization.
data "glitchtip_projects" "all" {
  organization = "acme-inc"
}

output "project_slugs" {
  value = [for p in data.glitchtip_projects.all.projects : p.slug]
}

# Example use: attach an alert to every existing project.
resource "glitchtip_project_alert" "errors" {
  for_each = { for p in data.glitchtip_projects.all.projects : p.slug => p }

  organization     = "acme-inc"
  project          = each.value.slug
  name             = "High error rate"
  timespan_minutes = 10
  quantity         = 100

  alert_recipients = [
    { recipient_type = "email" },
  ]
}
