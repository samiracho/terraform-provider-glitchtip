#!/usr/bin/env bash
#
# Tear down the `make demo-up` environment: stop and wipe the GlitchTip stack
# (which deletes all data) and remove local Terraform/dev-override artifacts.
set -euo pipefail

cd "$(dirname "$0")/.."

echo "==> Stopping and wiping the GlitchTip stack..."
docker compose -f test/docker-compose.yml down -v --remove-orphans 2>/dev/null || true

echo "==> Cleaning local artifacts..."
rm -f terraform-provider-glitchtip /tmp/glitchtip-demo.tfrc
rm -rf examples/demo/.terraform examples/demo/.terraform.lock.hcl examples/demo/terraform.tfstate examples/demo/terraform.tfstate.backup

echo "    done."
