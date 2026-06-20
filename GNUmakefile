default: build

BINARY=terraform-provider-glitchtip

# Build the provider binary.
.PHONY: build
build:
	go build -o $(BINARY)

# Install the provider into $GOPATH/bin.
.PHONY: install
install:
	go install

# Format Go source.
.PHONY: fmt
fmt:
	gofmt -s -w .

# Run go vet.
.PHONY: vet
vet:
	go vet ./...

# Run unit tests (no live GlitchTip instance required).
.PHONY: test
test:
	go test ./... -timeout=120s

# Run acceptance tests. Requires GLITCHTIP_TOKEN (and optionally GLITCHTIP_ENDPOINT)
# to point at a real GlitchTip instance. These create and destroy real resources.
.PHONY: testacc
testacc:
	TF_ACC=1 go test ./... -v -timeout=120m

# Run end-to-end tests against a throwaway GlitchTip launched with docker compose.
# Spins up the stack, mints an API token, runs the acceptance tests, and tears
# the stack down. Set KEEP_UP=1 to leave the stack running afterwards.
.PHONY: test-e2e
test-e2e:
	./scripts/run-e2e.sh

# Start / stop the local GlitchTip stack without running tests.
.PHONY: e2e-up e2e-down
e2e-up:
	docker compose -f test/docker-compose.yml up -d
e2e-down:
	docker compose -f test/docker-compose.yml down -v --remove-orphans

# Stand up a local GlitchTip and `terraform apply` the examples/demo config
# (without destroying it) so the created resources can be browsed in the web UI.
# Prints the URL and login. Tear down with `make demo-down`.
.PHONY: demo-up demo-down
demo-up:
	./scripts/demo-up.sh
demo-down:
	./scripts/demo-down.sh

# Lint with golangci-lint (must be installed separately).
.PHONY: lint
lint:
	golangci-lint run

# Regenerate documentation under docs/ from schema descriptions and examples/.
.PHONY: docs
docs:
	go generate ./...
