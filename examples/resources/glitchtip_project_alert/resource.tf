resource "glitchtip_project_alert" "example" {
  organization     = glitchtip_organization.example.slug
  project          = glitchtip_project.example.slug
  name             = "High error rate"
  timespan_minutes = 10
  quantity         = 100

  alert_recipients = [
    {
      recipient_type = "email"
    },
    {
      recipient_type = "webhook"
      url            = "https://hooks.example.com/glitchtip"
    },
  ]
}
