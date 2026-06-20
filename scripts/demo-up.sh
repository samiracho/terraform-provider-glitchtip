#!/usr/bin/env bash
#
# Stand up a local GlitchTip and `terraform apply` the examples/demo config
# against it WITHOUT destroying, so the resources can be browsed in the web UI.
# Leaves everything running; tear down with `make demo-down`.
set -euo pipefail

cd "$(dirname "$0")/.."

export GLITCHTIP_HOST_PORT="${GLITCHTIP_HOST_PORT:-8123}"
COMPOSE=(docker compose -f test/docker-compose.yml)
ENDPOINT="http://localhost:${GLITCHTIP_HOST_PORT}"
TFRC=/tmp/glitchtip-demo.tfrc

echo "==> Starting GlitchTip..."
"${COMPOSE[@]}" up -d

echo "==> Waiting for the API at ${ENDPOINT}..."
for i in $(seq 1 120); do
  code="$(curl -s -o /dev/null -w '%{http_code}' "${ENDPOINT}/api/0/organizations/" || echo 000)"
  [[ "$code" =~ ^[2-4][0-9][0-9]$ ]] && break
  sleep 3
done

echo "==> Creating admin user + API token..."
out="$("${COMPOSE[@]}" exec -T web python manage.py shell -c "$(cat test/bootstrap.py)" 2>/dev/null)"
token="$(printf '%s' "$out" | sed -n 's/^GLITCHTIP_TOKEN=//p' | tr -d '\r\n')"
email="$(printf '%s' "$out" | sed -n 's/^GLITCHTIP_LOGIN_EMAIL=//p' | tr -d '\r\n')"
pw="$(printf '%s' "$out" | sed -n 's/^GLITCHTIP_LOGIN_PASSWORD=//p' | tr -d '\r\n')"
if [[ -z "$token" ]]; then echo "ERROR: failed to mint token" >&2; exit 1; fi

echo "==> Building provider + dev override..."
go build -o terraform-provider-glitchtip .
cat > "$TFRC" <<EOF
provider_installation {
  dev_overrides {
    "samiracho/glitchtip" = "$PWD"
  }
  direct {}
}
EOF

echo "==> terraform apply (examples/demo)..."
TF_CLI_CONFIG_FILE="$TFRC" GLITCHTIP_ENDPOINT="$ENDPOINT" GLITCHTIP_TOKEN="$token" \
  terraform -chdir=examples/demo apply -auto-approve

cat <<EOF

============================================================
 GlitchTip is populated and running — open it in your browser:

   URL:      ${ENDPOINT}
   Login:    ${email}
   Password: ${pw}

 Where to look:
   - Org switcher (top-left): "Demo Org"
   - Projects: api, web
   - Project "api" -> Settings -> Client Keys (DSN): production
   - Project "api" -> Alerts: High error rate
   - Uptime Monitors: API health
   - Settings -> Teams: backend

 Tear it all down with:  make demo-down
============================================================
EOF
