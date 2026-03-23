#!/usr/bin/env bash
set -euo pipefail

CLUSTER_NAME="${CLUSTER_NAME:-headtotails}"
KIND_PROVIDER="${KIND_PROVIDER:-auto}"
KIND_KUBECONFIG="${KIND_KUBECONFIG:-.kind/${CLUSTER_NAME}.kubeconfig}"
HEADSCALE_NS="${HEADSCALE_NS:-headscale}"
LOCAL_CONTROL_PORT="${LOCAL_CONTROL_PORT:-18080}"
PF_PID_FILE="${PF_PID_FILE:-.kind/control-router.port-forward.pid}"
PF_LOG_FILE="${PF_LOG_FILE:-.kind/control-router.port-forward.log}"

mkdir -p .kind

if ! command -v kubectl >/dev/null 2>&1; then
  echo "kubectl is required (run via: nix develop -c scripts/kind-local-up.sh)"
  exit 1
fi

echo "Ensuring Kind cluster exists..."
KIND_PROVIDER="${KIND_PROVIDER}" KIND_KUBECONFIG="${KIND_KUBECONFIG}" scripts/kind-up.sh

echo "Ensuring headscale + headtotails + operator are deployed..."
KIND_PROVIDER="${KIND_PROVIDER}" KIND_KUBECONFIG="${KIND_KUBECONFIG}" scripts/kind-deploy-stack.sh

export KUBECONFIG="${KIND_KUBECONFIG}"

if [[ -f "${PF_PID_FILE}" ]]; then
  old_pid="$(cat "${PF_PID_FILE}")"
  if kill -0 "${old_pid}" >/dev/null 2>&1; then
    echo "Local port-forward already running (pid=${old_pid})"
  else
    rm -f "${PF_PID_FILE}"
  fi
fi

if [[ ! -f "${PF_PID_FILE}" ]]; then
  echo "Starting local control-router port-forward on localhost:${LOCAL_CONTROL_PORT}..."
  nohup kubectl -n "${HEADSCALE_NS}" \
    port-forward svc/control-router "${LOCAL_CONTROL_PORT}:80" --address 127.0.0.1 \
    >"${PF_LOG_FILE}" 2>&1 &
  pf_pid=$!
  echo "${pf_pid}" > "${PF_PID_FILE}"
  sleep 2
  if ! kill -0 "${pf_pid}" >/dev/null 2>&1; then
    echo "Failed to start port-forward. Log output:"
    if [[ -f "${PF_LOG_FILE}" ]]; then
      sed -n '1,120p' "${PF_LOG_FILE}"
    fi
    rm -f "${PF_PID_FILE}"
    exit 1
  fi
fi

base_url="http://127.0.0.1:${LOCAL_CONTROL_PORT}"
echo ""
echo "Local endpoint is ready:"
echo "  ${base_url}"
echo ""
echo "In another terminal, export whichever your client needs:"
echo "  export TAILSCALE_API_URL=${base_url}"
echo "  export OPERATOR_LOGIN_SERVER=${base_url}"
echo "  export HEADTOTAILS_BASE_URL=${base_url}"
echo ""
echo "Status helpers:"
echo "  make kind-local-status"
echo "  make kind-local-down"
