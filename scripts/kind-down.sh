#!/usr/bin/env bash
set -euo pipefail

CLUSTER_NAME="${CLUSTER_NAME:-headtotails}"
KIND_PROVIDER="${KIND_PROVIDER:-auto}"

if [[ "${KIND_PROVIDER}" == "podman" ]] || { [[ "${KIND_PROVIDER}" == "auto" ]] && command -v podman >/dev/null 2>&1; }; then
  export KIND_EXPERIMENTAL_PROVIDER=podman
fi

if ! command -v kind >/dev/null 2>&1; then
  echo "kind is required (run via: nix develop -c scripts/kind-down.sh)"
  exit 1
fi

if kind get clusters | rg -qx "${CLUSTER_NAME}"; then
  echo "Deleting Kind cluster '${CLUSTER_NAME}'..."
  kind delete cluster --name "${CLUSTER_NAME}"
else
  echo "Kind cluster '${CLUSTER_NAME}' does not exist"
fi
