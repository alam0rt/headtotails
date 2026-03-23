#!/usr/bin/env bash
set -euo pipefail

CLUSTER_NAME="${CLUSTER_NAME:-headtotails}"
OPERATOR_RELEASE="${OPERATOR_RELEASE:-tailscale-operator}"
OPERATOR_NS="${OPERATOR_NS:-tailscale}"
HEADSCALE_NS="${HEADSCALE_NS:-headscale}"
KIND_PROVIDER="${KIND_PROVIDER:-auto}"
KIND_KUBECONFIG="${KIND_KUBECONFIG:-.kind/${CLUSTER_NAME}.kubeconfig}"

OAUTH_CLIENT_ID="${OAUTH_CLIENT_ID:-kind-operator}"
OAUTH_CLIENT_SECRET="${OAUTH_CLIENT_SECRET:-kind-operator-secret}"
OAUTH_HMAC_SECRET="${OAUTH_HMAC_SECRET:-kind-shared-hmac-secret-32-bytes-min}"
TAILNET_NAME="${TAILNET_NAME:-tailscale-operator}"
HEADTOTAILS_IMAGE="${HEADTOTAILS_IMAGE:-localhost/headtotails:kind}"

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

for bin in kind kubectl helm rg; do
  if ! command -v "${bin}" >/dev/null 2>&1; then
    echo "Missing required dependency: ${bin}"
    echo "Run through nix dev shell, for example:"
    echo "  nix develop -c scripts/kind-deploy-stack.sh"
    exit 1
  fi
done

if [[ "${PROVIDER}" == "none" ]]; then
  echo "No container runtime found. Install podman or docker."
  exit 1
fi

if [[ "${PROVIDER}" == "podman" ]]; then
  export KIND_EXPERIMENTAL_PROVIDER=podman
  if ! command -v podman >/dev/null 2>&1; then
    echo "Podman was selected but binary not found."
    exit 1
  fi
  if ! podman info >/dev/null 2>&1; then
    echo "Podman is installed but not ready. Ensure podman machine/socket is running."
    exit 1
  fi
  IMAGE_TOOL="podman"
else
  if ! command -v docker >/dev/null 2>&1; then
    echo "Docker was selected but binary not found."
    exit 1
  fi
  if ! docker info >/dev/null 2>&1; then
    echo "Docker is installed but not ready. Ensure the Docker daemon is running."
    exit 1
  fi
  IMAGE_TOOL="docker"
fi

echo "Using Kind provider: ${PROVIDER}"
export KUBECONFIG="${KIND_KUBECONFIG}"
if [[ ! -f "${KUBECONFIG}" ]]; then
  echo "Kubeconfig '${KUBECONFIG}' not found. Run scripts/kind-up.sh first."
  exit 1
fi

if ! kind get clusters | rg -qx "${CLUSTER_NAME}"; then
  echo "Kind cluster '${CLUSTER_NAME}' does not exist. Run scripts/kind-up.sh first."
  exit 1
fi

echo "Building local headtotails image..."
"${IMAGE_TOOL}" build -t "${HEADTOTAILS_IMAGE}" .
image_archive="$(mktemp -t headtotails-kind-image-XXXXXX.tar)"
trap 'rm -f "${image_archive}"' EXIT
"${IMAGE_TOOL}" save -o "${image_archive}" "${HEADTOTAILS_IMAGE}"
kind load image-archive --name "${CLUSTER_NAME}" "${image_archive}"

echo "Deploying headscale..."
kubectl apply -f deploy/kind/headscale.yaml
kubectl -n "${HEADSCALE_NS}" rollout status deploy/headscale --timeout=180s

echo "Ensuring operator user exists in headscale..."
user_create_out="$(mktemp)"
if ! kubectl -n "${HEADSCALE_NS}" exec deploy/headscale -- \
  headscale users create "${TAILNET_NAME}" >"${user_create_out}" 2>&1; then
  if ! rg -qi "already exists|unique constraint failed|constraint failed" "${user_create_out}"; then
    echo "Failed to create headscale user '${TAILNET_NAME}'"
    rg "." "${user_create_out}" || true
    rm -f "${user_create_out}"
    exit 1
  fi
fi
rm -f "${user_create_out}"

echo "Creating headscale API key..."
api_key_raw="$(kubectl -n "${HEADSCALE_NS}" exec deploy/headscale -- \
  headscale apikeys create --expiration 24h)"
HEADSCALE_API_KEY="$(printf '%s' "${api_key_raw}" | rg -o 'hskey-[^[:space:]]+' -m1)"
if [[ -z "${HEADSCALE_API_KEY}" ]]; then
  echo "Failed to parse headscale API key from output:"
  printf '%s\n' "${api_key_raw}"
  exit 1
fi

echo "Creating headtotails secret..."
kubectl -n "${HEADSCALE_NS}" create secret generic headtotails-secrets \
  --from-literal=headscale_api_key="${HEADSCALE_API_KEY}" \
  --from-literal=oauth_client_id="${OAUTH_CLIENT_ID}" \
  --from-literal=oauth_client_secret="${OAUTH_CLIENT_SECRET}" \
  --from-literal=oauth_hmac_secret="${OAUTH_HMAC_SECRET}" \
  --from-literal=tailnet_name="${TAILNET_NAME}" \
  --dry-run=client -o yaml | kubectl apply -f -

echo "Deploying headtotails and control router..."
kubectl apply -f deploy/kind/headtotails.yaml
kubectl apply -f deploy/kind/control-router.yaml
kubectl -n "${HEADSCALE_NS}" rollout status deploy/headtotails --timeout=180s
kubectl -n "${HEADSCALE_NS}" rollout status deploy/control-router --timeout=180s

echo "Installing/upgrading Tailscale operator..."
helm repo add tailscale https://pkgs.tailscale.com/helmcharts >/dev/null 2>&1 || true
helm repo update >/dev/null
helm upgrade --install "${OPERATOR_RELEASE}" tailscale/tailscale-operator \
  --namespace "${OPERATOR_NS}" \
  --create-namespace \
  --set-string oauth.clientId="${OAUTH_CLIENT_ID}" \
  --set-string oauth.clientSecret="${OAUTH_CLIENT_SECRET}" \
  --set-string loginServer="http://control-router.${HEADSCALE_NS}.svc.cluster.local"

kubectl -n "${OPERATOR_NS}" rollout status deploy/operator --timeout=240s

echo "Stack deployed successfully"
