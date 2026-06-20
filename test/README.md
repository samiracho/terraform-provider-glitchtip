# End-to-end tests

This directory contains a throwaway GlitchTip stack used to run the provider's
acceptance tests against a real instance.

## Run

```sh
make test-e2e
# or
./scripts/run-e2e.sh
```

This will:

1. `docker compose up` a local GlitchTip ([test/docker-compose.yml](docker-compose.yml)):
   Postgres + an all-in-one GlitchTip web container (Valkey/Redis is disabled —
   GlitchTip uses Postgres for the queue/cache). The runner publishes it on
   `http://localhost:8123` by default (override with `GLITCHTIP_HOST_PORT`).
2. Wait for migrations and the API to be ready.
3. Create a verified admin user and mint a fully-scoped API token via
   [test/bootstrap.py](bootstrap.py) (`manage.py shell`). The runner prints the
   web-UI login credentials.
4. Run `TF_ACC=1 go test ./internal/provider/...` with `GLITCHTIP_ENDPOINT` and
   `GLITCHTIP_TOKEN` pointed at the local instance.
5. Tear the stack down (`docker compose down -v`).

## Options

- `KEEP_UP=1 make test-e2e` — leave the stack running after tests (inspect it at
  `http://localhost:8123`, tear down later with `make e2e-down`).
- `./scripts/run-e2e.sh -run TestAccTeamResource_basic` — run a subset; any extra
  arguments are passed through to `go test`.
- `make e2e-up` / `make e2e-down` — start/stop the stack without running tests.

The acceptance tests require a `terraform` (or `tofu`) binary; the
terraform-plugin-testing framework downloads one automatically if none is found
on `PATH`.

## Browsing created resources in the web UI

The acceptance tests destroy everything they create, so there is nothing left to
look at afterwards. To see provider-managed resources in the GlitchTip web UI,
apply a config and leave it running:

```sh
make demo-up     # boots GlitchTip, applies examples/demo, prints URL + login
# ... open the printed URL, log in, browse Demo Org / projects / alerts / monitors ...
make demo-down   # destroys everything (wipes the stack)
```

`make demo-up` runs a real `terraform apply` of [examples/demo/main.tf](../examples/demo/main.tf)
against the local instance using a dev-override build of the provider, and does
**not** destroy it. Because the API token and the web-UI login are the same user,
the org/projects/keys/alerts/monitor you create show up under that account.

## Logging into the web UI

GlitchTip ships with no default account. The bootstrap step creates a verified
**superuser** you can log in with at `http://localhost:8123`:

| Field    | Default                  | Override      |
| -------- | ------------------------ | ------------- |
| email    | `e2e@example.com`        | `E2E_EMAIL`   |
| password | `e2e-password-12345!`    | `E2E_PASSWORD`|

The runner prints these after minting the token. If you started the stack
yourself (`make e2e-up`), run the bootstrap to create the user:

```sh
docker compose -f test/docker-compose.yml exec -T web \
  python manage.py shell -c "$(cat test/bootstrap.py)"
```

The email is marked verified, so password login works without the confirmation
step. The account is also a Django superuser; set `ENABLE_ADMIN: "True"` in the
compose file to reach the Django admin at `/admin/`.

> Note: teardown runs `docker compose down -v`, which wipes the database — the
> user only exists between an `e2e-up` + bootstrap and the next teardown.

> This stack is for testing only — it trusts all Postgres connections, uses a
> throwaway secret key, and opens user registration so a token can be minted
> automatically. Do not expose it publicly.
