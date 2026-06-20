# Terraform Provider for GlitchTip

A [Terraform](https://www.terraform.io) / [OpenTofu](https://opentofu.org)
provider for managing [GlitchTip](https://glitchtip.com) — the open-source,
Sentry-compatible error tracking and uptime monitoring platform — as code.

It is built with the
[Terraform Plugin Framework](https://developer.hashicorp.com/terraform/plugin/framework)
and talks to the GlitchTip REST API
([API docs](https://app.glitchtip.com/api/docs)).


## Features

### Resources

| Resource                         | Description                                               |
| -------------------------------- | --------------------------------------------------------- |
| `glitchtip_organization`         | An organization (the top-level tenant).                   |
| `glitchtip_team`                 | A team within an organization.                            |
| `glitchtip_project`              | A project (an error-ingestion endpoint), created via a team. |
| `glitchtip_project_key`          | A project key / DSN used to ingest events.                |
| `glitchtip_project_alert`        | An alert (notification rule) with email/webhook recipients. |
| `glitchtip_organization_member`  | An organization member invitation and its role.           |
| `glitchtip_monitor`              | An uptime monitor.                                        |
| `glitchtip_project_team`         | Associates an existing project with an additional team.   |

### Data sources

| Data source                       | Description                                  |
| --------------------------------- | -------------------------------------------- |
| `glitchtip_organization`          | Look up an organization by slug.             |
| `glitchtip_team`                  | Look up a team by org + slug.                |
| `glitchtip_project`               | Look up a project by org + slug.             |
| `glitchtip_organizations`         | List all accessible organizations.           |
| `glitchtip_projects`              | List all projects in an organization.        |
| `glitchtip_teams`                 | List all teams in an organization.           |
| `glitchtip_organization_members`  | List all members of an organization.         |
| `glitchtip_monitors`              | List all uptime monitors in an organization. |
| `glitchtip_project_keys`          | List all keys (DSNs) of a project.           |

### Resource identity & bulk discovery

Every resource implements [resource identity](https://developer.hashicorp.com/terraform/plugin/framework/resources/identity),
so they can be imported by identity (Terraform 1.12+):

```hcl
import {
  to = glitchtip_project.api
  identity = {
    organization = "acme-inc"
    id           = "42" # stable numeric id (slug is mutable, so identity uses id)
  }
}
```

The org-level resources also provide [list resources](https://developer.hashicorp.com/terraform/plugin/framework/list-resources)
for `terraform query` (Terraform 1.14+), to discover and bulk-import existing
objects (`glitchtip_organization`, `_team`, `_project`, `_project_key`,
`_project_alert`, `_organization_member`, `_monitor`):

```hcl
# list.tfquery.hcl
list "glitchtip_project" "all" {
  provider = glitchtip
  config { organization = "acme-inc" }
}
```
```sh
terraform query   # lists every project with its identity, ready to import
```

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0, or OpenTofu >= 1.6
- [Go](https://go.dev/dl/) >= 1.24 (to build the provider)
- A GlitchTip instance and an API token (created under **Profile → Auth Tokens**).

## Usage

```hcl
terraform {
  required_providers {
    glitchtip = {
      source = "samiracho/glitchtip"
    }
  }
}

provider "glitchtip" {
  endpoint = "https://glitchtip.example.com" # defaults to https://app.glitchtip.com
  token    = var.glitchtip_token             # or set GLITCHTIP_TOKEN
}

resource "glitchtip_organization" "acme" {
  name = "Acme Inc"
}

resource "glitchtip_team" "backend" {
  organization = glitchtip_organization.acme.slug
  slug         = "backend"
}

resource "glitchtip_project" "api" {
  organization = glitchtip_organization.acme.slug
  team         = glitchtip_team.backend.slug
  name         = "api"
  platform     = "python"
}

resource "glitchtip_project_key" "api" {
  organization = glitchtip_organization.acme.slug
  project      = glitchtip_project.api.slug
  name         = "production"
}

output "dsn" {
  value     = glitchtip_project_key.api.dsn
  sensitive = true
}
```

### Provider configuration

| Argument   | Environment variable  | Default                       | Description                                  |
| ---------- | --------------------- | ----------------------------- | -------------------------------------------- |
| `endpoint` | `GLITCHTIP_ENDPOINT`  | `https://app.glitchtip.com`   | Base URL of the GlitchTip instance.          |
| `token`    | `GLITCHTIP_TOKEN`     | —                             | API authentication token (required).         |

## Development

```sh
make build      # build the provider binary
make test       # run unit tests (no live instance required)
make fmt vet    # format and vet
make lint       # golangci-lint (must be installed)
make docs       # regenerate docs/ from schemas + examples/
```

### Acceptance tests

Acceptance tests create and destroy **real** resources against a live GlitchTip
instance. Provide credentials via environment variables and run:

```sh
export GLITCHTIP_ENDPOINT="https://glitchtip.example.com"
export GLITCHTIP_TOKEN="your-api-token"
make testacc
```

### Running against a local build

```sh
go build -o terraform-provider-glitchtip
```

Add a [dev override](https://developer.hashicorp.com/terraform/cli/config/config-file#development-overrides-for-provider-developers)
to `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "samiracho/glitchtip" = "/path/to/repo"
  }
  direct {}
}
```

## License

MIT — see [LICENSE](LICENSE).
