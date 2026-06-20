resource "glitchtip_monitor" "example" {
  organization    = glitchtip_organization.example.slug
  name            = "API health"
  monitor_type    = "GET"
  url             = "https://api.example.com/health"
  interval        = 60
  timeout         = 20
  expected_status = 200

  # Optionally associate the monitor with a project (by numeric id).
  project_id = glitchtip_project.example.id
}
