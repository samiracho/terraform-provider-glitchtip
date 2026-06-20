# List all keys (DSNs) of a project.
data "glitchtip_project_keys" "all" {
  organization = "acme-inc"
  project      = "api"
}

output "dsns" {
  value     = [for k in data.glitchtip_project_keys.all.keys : k.dsn["public"]]
  sensitive = true
}
