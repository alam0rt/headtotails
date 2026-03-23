#!/usr/bin/env bash
set -euo pipefail

CLUSTER_NAME="${CLUSTER_NAME:-headtotails}"
KIND_PROVIDER="${KIND_PROVIDER:-auto}"
KIND_KUBECONFIG="${KIND_KUBECONFIG:-.kind/${CLUSTER_NAME}.kubeconfig}"

pick_provider() {
  if [[ "${KIND_PROVIDER}" != "auto" ]]; then
    printf '%s' "${KIND_PROVIDER}"
    return 0
  fi

  if command -v podman >/dev/null 2>&1; then
    printf 'podman'
    return 0
  fi

  if command -v docker >/dev/null 2>&1; then
    printf 'docker'
    return 0
  fi

  printf 'none'
}

PROVIDER="$(pick_provider)"

if ! command -v kind >/dev/null 2>&1; then
  echo "kind is required (run via: nix develop -c scripts/kind-up.sh)"
  exit 1
fi

if ! command -v kubectl >/dev/null 2>&1; then
  echo "kubectl is required (run via: nix develop -c scripts/kind-up.sh)"
  exit 1
fi

if [[ "${PROVIDER}" == "none" ]]; then
  echo "No container runtime found. Install podman or docker."
  exit 1
fi

if [[ "${PROVIDER}" == "podman" ]]; then
  export KIND_EXPERIMENTAL_PROVIDER=podman
  if ! podman info >/dev/null 2>&1; then
    echo "Podman is installed but not ready. Ensure podman machine/socket is running."
    exit 1
  fi
else
  if ! docker info >/dev/null 2>&1; then
    echo "Docker is installed but not ready. Ensure the Docker daemon is running."
    exit 1
  fi
fi

echo "Using Kind provider: ${PROVIDER}"
mkdir -p "$(dirname "${KIND_KUBECONFIG}")"
export KUBECONFIG="${KIND_KUBECONFIG}"

if kind get clusters | rg -qx "${CLUSTER_NAME}"; then
  echo "Kind cluster '${CLUSTER_NAME}' already exists"
  kind export kubeconfig --name "${CLUSTER_NAME}" --kubeconfig "${KIND_KUBECONFIG}"
else
  echo "Creating Kind cluster '${CLUSTER_NAME}'..."
  kind create cluster --name "${CLUSTER_NAME}" --wait 120s --kubeconfig "${KIND_KUBECONFIG}"
fi

kubectl cluster-info >/dev/null
echo "Kind cluster '${CLUSTER_NAME}' is ready"
