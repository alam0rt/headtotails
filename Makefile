.PHONY: build test generate lint docker-build integration-test clean install-hooks kind-up kind-down kind-deploy kind-smoke-test kind-e2e kind-local-up kind-local-down kind-local-status

# Use CGO_ENABLED=0 for portability; the race detector requires gcc.
BUILD_FLAGS ?= CGO_ENABLED=0 GOFLAGS="-mod=mod"
BUILD_VERSION ?= dev
BINARY      := headtotails
IMAGE := headtotails:latest

## build: Compile the headtotails binary.
build:
	$(BUILD_FLAGS) go build -ldflags="-s -w -X main.version=$(BUILD_VERSION)" -o $(BINARY) ./cmd/headtotails

## test: Run all unit tests.
test:
	$(BUILD_FLAGS) go test ./internal/... ./cmd/...

## test-race: Run unit tests with the data race detector (requires gcc/cgo).
test-race:
	GOFLAGS="-mod=mod" go test -race ./internal/... ./cmd/...

## generate: Regenerate mocks (requires mockery v2 on PATH).
generate:
	go generate ./...
	mockery --name HeadscaleClient --dir internal/headscale \
	        --structname MockHeadscaleClient \
	        --output internal/headscale --outpkg headscale \
	        --filename mock_client.go

## lint: Run static analysis.
lint:
	golangci-lint run --timeout=5m

## docker-build: Build the Docker image.
docker-build:
	docker build -t $(IMAGE) .

## integration-test: Run integration tests against a real headscale Docker instance.
integration-test:
	HEADSCALE_INTEGRATION_TEST=1 go test -v -timeout 30m -count=1 ./integration/...

## clean: Remove build artifacts.
clean:
	rm -f $(BINARY)

## install-hooks: Install repository pre-commit hook into .git/hooks.
install-hooks:
	mkdir -p .git/hooks
	cp scripts/pre-commit .git/hooks/pre-commit
	chmod +x .git/hooks/pre-commit

## kind-up: Create local Kind cluster for operator testing (uses flake dev shell).
kind-up:
	nix develop -c scripts/kind-up.sh

## kind-down: Delete local Kind cluster used for operator testing.
kind-down:
	nix develop -c scripts/kind-down.sh

## kind-deploy: Deploy headscale + headtotails + operator into Kind.
kind-deploy:
	nix develop -c scripts/kind-deploy-stack.sh

## kind-smoke-test: Run smoke tests against the local Kind stack.
kind-smoke-test:
	nix develop -c scripts/kind-smoke-test.sh

## kind-e2e: Full local flow (up, deploy, smoke test).
kind-e2e:
	nix develop -c scripts/kind-up.sh
	nix develop -c scripts/kind-deploy-stack.sh
	nix develop -c scripts/kind-smoke-test.sh

## kind-local-up: Bring stack up and expose localhost control-router URL.
kind-local-up:
	nix develop -c scripts/kind-local-up.sh

## kind-local-down: Stop localhost control-router port-forward.
kind-local-down:
	nix develop -c scripts/kind-local-down.sh

## kind-local-status: Show localhost endpoint and port-forward status.
kind-local-status:
	nix develop -c scripts/kind-local-status.sh
