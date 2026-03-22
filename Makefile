.PHONY: build test generate lint docker-build integration-test clean

# Use CGO_ENABLED=0 for portability; the race detector requires gcc.
BUILD_FLAGS ?= CGO_ENABLED=0 GOFLAGS="-mod=mod"
BINARY      := headtotails
IMAGE := headtotails:latest

## build: Compile the headtotails binary.
build:
	$(BUILD_FLAGS) go build -ldflags="-s -w" -o $(BINARY) ./cmd/headtotails

## test: Run all unit tests.
test:
	$(BUILD_FLAGS) go test ./internal/... ./cmd/...

## test-race: Run unit tests with the data race detector (requires gcc/cgo).
test-race:
	GOFLAGS="-mod=mod" go test -race ./internal/... ./cmd/...

## generate: Regenerate mocks (requires mockery v2 on PATH).
generate:
	mockery --name HeadscaleClient --dir internal/headscale \
	        --output internal/headscale --outpkg headscale \
	        --filename mock_client.go

## lint: Run static analysis.
lint:
	go vet ./...

## docker-build: Build the Docker image.
docker-build:
	docker build -t $(IMAGE) .

## integration-test: Run integration tests against a real headscale Docker instance.
integration-test:
	HEADSCALE_INTEGRATION_TEST=1 go test -v -timeout 30m -count=1 ./integration/...

## clean: Remove build artifacts.
clean:
	rm -f $(BINARY)
