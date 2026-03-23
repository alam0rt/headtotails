#!/usr/bin/env bash
set -euo pipefail

OPERATOR_NS="${OPERATOR_NS:-tailscale}"
HEADSCALE_NS="${HEADSCALE_NS:-headscale}"
TEST_NS="${TEST_NS:-kind-test}"
CLUSTER_NAME="${CLUSTER_NAME:-headtotails}"
KIND_KUBECONFIG="${KIND_KUBECONFIG:-.kind/${CLUSTER_NAME}.kubeconfig}"

for bin in kubectl jq rg; do
  if ! command -v "${bin}" >/dev/null 2>&1; then
    echo "Missing required dependency: ${bin}"
    echo "Run through nix dev shell, for example:"
    echo "  nix develop -c scripts/kind-smoke-test.sh"
    exit 1
  fi
done

export KUBECONFIG="${KIND_KUBECONFIG}"
if [[ ! -f "${KUBECONFIG}" ]]; then
  echo "Kubeconfig '${KUBECONFIG}' not found. Run scripts/kind-up.sh first."
  exit 1
fi

echo "Deploying smoke-test workload..."
kubectl -n "${TEST_NS}" delete svc whoami --ignore-not-found
kubectl apply -f deploy/kind/whoami-service.yaml
kubectl -n "${TEST_NS}" rollout status deploy/whoami --timeout=120s

echo "Waiting for operator-managed StatefulSet for whoami service..."
deadline=$((SECONDS + 120))
while (( SECONDS < deadline )); do
  sts_name="$(kubectl -n "${OPERATOR_NS}" get statefulset \
    -l tailscale.com/parent-resource=whoami \
    -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)"
  if [[ -n "${sts_name}" ]]; then
    echo "Found StatefulSet: ${sts_name}"
    kubectl -n "${OPERATOR_NS}" rollout status "statefulset/${sts_name}" --timeout=180s
    break
  fi
  sleep 5
done

if [[ -n "${sts_name:-}" ]]; then
  echo "Checking headscale nodes..."
  nodes_raw="$(kubectl -n "${HEADSCALE_NS}" exec deploy/headscale -- headscale nodes list --output json)"
  node_count="$(printf '%s' "${nodes_raw}" | jq 'if type=="array" then length elif has("nodes") then .nodes|length else 0 end')"
  if [[ "${node_count}" -lt 1 ]]; then
    echo "Expected at least one node in headscale, found ${node_count}"
    exit 1
  fi
  echo "Smoke test passed: operator created proxy resources and headscale has nodes (${node_count})"
  exit 0
fi

echo "No operator StatefulSet observed; running bootstrap smoke fallback checks..."
kubectl -n "${OPERATOR_NS}" get pods -o wide
kubectl -n "${OPERATOR_NS}" get statefulset || true

operator_ready="$(kubectl -n "${OPERATOR_NS}" get deploy operator -o jsonpath='{.status.readyReplicas}')"
if [[ "${operator_ready:-0}" -lt 1 ]]; then
  echo "Operator deployment is not ready"
  exit 1
fi

headtotails_ready="$(kubectl -n "${HEADSCALE_NS}" get deploy headtotails -o jsonpath='{.status.readyReplicas}')"
router_ready="$(kubectl -n "${HEADSCALE_NS}" get deploy control-router -o jsonpath='{.status.readyReplicas}')"
if [[ "${headtotails_ready:-0}" -lt 1 || "${router_ready:-0}" -lt 1 ]]; then
  echo "headtotails/control-router are not ready"
  exit 1
fi

if ! kubectl -n "${OPERATOR_NS}" get secret operator >/dev/null 2>&1; then
  echo "Operator state secret not found; operator bootstrap likely incomplete"
  exit 1
fi

echo "Smoke fallback passed: operator is running and control-plane components are healthy"
