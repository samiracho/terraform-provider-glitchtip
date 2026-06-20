# Self-contained demo applied by `make demo-up` to populate a local GlitchTip so
# you can browse the created resources in the web UI. Endpoint and token come
# from GLITCHTIP_ENDPOINT / GLITCHTIP_TOKEN (set by scripts/demo-up.sh).
terraform {
  required_providers {
    glitchtip = {
      source = "samiracho/glitchtip"
    }
  }
}

provider "glitchtip" {}

resource "glitchtip_organization" "demo" {
  name = "Demo Org"
}

resource "glitchtip_team" "backend" {
  organization = glitchtip_organization.demo.slug
  slug         = "backend"
}

resource "glitchtip_project" "api" {
  organization = glitchtip_organization.demo.slug
  team         = glitchtip_team.backend.slug
  name         = "api"
  platform     = "python"
}

resource "glitchtip_project" "web" {
  organization = glitchtip_organization.demo.slug
  team         = glitchtip_team.backend.slug
  name         = "web"
  platform     = "javascript"
}

resource "glitchtip_project_key" "api" {
  organization = glitchtip_organization.demo.slug
  project      = glitchtip_project.api.slug
  name         = "production"
}

resource "glitchtip_project_alert" "api" {
  organization     = glitchtip_organization.demo.slug
  project          = glitchtip_project.api.slug
  name             = "High error rate"
  timespan_minutes = 10
  quantity         = 100

  alert_recipients = [
    { recipient_type = "email" },
  ]
}

resource "glitchtip_monitor" "api" {
  organization    = glitchtip_organization.demo.slug
  name            = "API health"
  monitor_type    = "GET"
  url             = "https://example.com/health"
  interval        = 60
  expected_status = 200
  project_id      = glitchtip_project.api.id
}

output "api_dsn" {
  value     = glitchtip_project_key.api.dsn["public"]
  sensitive = true
}
