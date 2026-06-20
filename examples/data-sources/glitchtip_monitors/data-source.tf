# List all uptime monitors in an organization.
data "glitchtip_monitors" "all" {
  organization = "acme-inc"
}

output "down_monitors" {
  value = [for m in data.glitchtip_monitors.all.monitors : m.name if m.is_up == false]
}
