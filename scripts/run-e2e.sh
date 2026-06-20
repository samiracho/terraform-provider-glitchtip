#!/usr/bin/env bash
#
# End-to-end test runner: starts a local GlitchTip with docker compose, mints a
# fully-scoped API token, and runs the provider acceptance tests against it.
#
# Usage:
#   scripts/run-e2e.sh                 # run all acceptance tests
#   scripts/run-e2e.sh -run TestAccTeam   # pass extra args through to `go test`
#
# Environment:
#   KEEP_UP=1   leave the GlitchTip stack running after the tests (skip teardown)
#   GLITCHTIP_IMAGE   override the glitchtip image tag (default from compose file)
set -euo pipefail

cd "$(dirname "$0")/.."

# Host port for the GlitchTip web container. Defaults to 8123 to avoid the
# common conflict on 8000; override with GLITCHTIP_HOST_PORT. It is exported so
# docker compose picks it up for both the port mapping and GLITCHTIP_DOMAIN.
export GLITCHTIP_HOST_PORT="${GLITCHTIP_HOST_PORT:-8123}"
COMPOSE=(docker compose -f test/docker-compose.yml)
ENDPOINT="http://localhost:${GLITCHTIP_HOST_PORT}"

cleanup() {
  if [[ "${KEEP_UP:-0}" == "1" ]]; then
    echo "==> KEEP_UP=1 set; leaving stack running at ${ENDPOINT}"
    if [[ -n "${login_email:-}" ]]; then
      echo "    web UI login: ${login_email} / ${login_pw}"
    fi
    echo "    tear down with: docker compose -f test/docker-compose.yml down -v"
  else
    echo "==> Tearing down GlitchTip stack..."
    "${COMPOSE[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

echo "==> Starting GlitchTip (this pulls images on first run)..."
"${COMPOSE[@]}" up -d

echo "==> Waiting for the API to come up at ${ENDPOINT} (migrations can take a minute)..."
ready=0
for i in $(seq 1 120); do
  code="$(curl -s -o /dev/null -w '%{http_code}' "${ENDPOINT}/api/0/organizations/" || echo 000)"
  # Any non-5xx, non-000 response means routing + DB migrations are ready
  # (401/403 unauthorized is exactly what we expect before authenticating).
  if [[ "$code" =~ ^[2-4][0-9][0-9]$ ]]; then
    ready=1
    echo "    ready (HTTP ${code})"
    break
  fi
  sleep 3
done
if [[ "$ready" != "1" ]]; then
  echo "ERROR: GlitchTip did not become ready in time. Recent web logs:" >&2
  "${COMPOSE[@]}" logs --tail=60 web >&2 || true
  exit 1
fi

echo "==> Minting a fully-scoped API token..."
token=""
for i in $(seq 1 10); do
  out="$("${COMPOSE[@]}" exec -T web python manage.py shell -c "$(cat test/bootstrap.py)" 2>/dev/null || true)"
  token="$(printf '%s' "$out" | sed -n 's/^GLITCHTIP_TOKEN=//p' | tr -d '\r\n')"
  [[ -n "$token" ]] && break
  sleep 3
done
if [[ -z "$token" ]]; then
  echo "ERROR: failed to create an API token. bootstrap output:" >&2
  "${COMPOSE[@]}" exec -T web python manage.py shell -c "$(cat test/bootstrap.py)" >&2 || true
  exit 1
fi
login_email="$(printf '%s' "$out" | sed -n 's/^GLITCHTIP_LOGIN_EMAIL=//p' | tr -d '\r\n')"
login_pw="$(printf '%s' "$out" | sed -n 's/^GLITCHTIP_LOGIN_PASSWORD=//p' | tr -d '\r\n')"
echo "    token created (${#token} chars)"
echo "    web UI login: ${login_email} / ${login_pw}"

echo "==> Verifying the token authenticates..."
curl -fsS -H "Authorization: Bearer ${token}" "${ENDPOINT}/api/0/organizations/" >/dev/null
echo "    ok"

echo "==> Running acceptance tests..."
export TF_ACC=1
export GLITCHTIP_ENDPOINT="${ENDPOINT}"
export GLITCHTIP_TOKEN="${token}"

# Default to all provider acceptance tests; allow callers to pass extra flags
# (e.g. -run TestAccTeam) which override the default -run.
if [[ "$*" == *"-run"* ]]; then
  go test ./internal/provider/... -v -timeout 30m "$@"
else
  go test ./internal/provider/... -v -timeout 30m -run 'TestAcc' "$@"
fi
