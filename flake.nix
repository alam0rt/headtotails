{
  description = "headtotails — Tailscale API v2 proxy for headscale";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        devShells.default = pkgs.mkShell {
          name = "headtotails";

          packages = with pkgs; [
            # Go toolchain — 1.25.x is the default `go` in nixpkgs-unstable
            go

            # C compiler — required for `go test -race`
            gcc

            # Build tooling
            gnumake
            git

            # Protobuf / gRPC code generation
            protobuf
            protoc-gen-go
            protoc-gen-go-grpc

            # Mock generation (vektra/mockery — installed via `go install` in shellHook
            # because nixpkgs does not ship mockery as a top-level package)

            # Linting / static analysis
            golangci-lint
            gotools          # goimports, godoc, etc.

            # Docker (for integration tests)
            docker-client

            # Useful extras
            jq
            curl
          ];

          shellHook = ''
            echo "headtotails dev shell"
            echo "  go      $(go version)"
            echo "  gcc     $(gcc --version | head -1)"
            echo "  make    $(make --version | head -1)"
            echo ""

            # Install mockery if not already present (vektra/mockery v2).
            if ! command -v mockery &>/dev/null; then
              echo "Installing mockery v2..."
              GOBIN="$HOME/.local/bin" go install github.com/vektra/mockery/v2@latest
              export PATH="$HOME/.local/bin:$PATH"
            fi

            echo "Run 'make test' to run unit tests."
            echo "Set HEADSCALE_INTEGRATION_TEST=1 and run 'make integration-test' for end-to-end tests."
          '';
        };
      }
    );
}
