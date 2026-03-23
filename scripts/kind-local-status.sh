#!/usr/bin/env bash
set -euo pipefail

CLUSTER_NAME="${CLUSTER_NAME:-headtotails}"
LOCAL_CONTROL_PORT="${LOCAL_CONTROL_PORT:-18080}"
PF_PID_FILE="${PF_PID_FILE:-.kind/control-router.port-forward.pid}"
PF_LOG_FILE="${PF_LOG_FILE:-.kind/control-router.port-forward.log}"

echo "Cluster: ${CLUSTER_NAME}"
echo "Local URL: http://127.0.0.1:${LOCAL_CONTROL_PORT}"

if [[ -f "${PF_PID_FILE}" ]]; then
  pid="$(cat "${PF_PID_FILE}")"
  if kill -0 "${pid}" >/dev/null 2>&1; then
    echo "Port-forward: running (pid=${pid})"
  else
    echo "Port-forward: stale pid file (pid=${pid})"
  fi
else
  echo "Port-forward: not running"
fi

if [[ -f "${PF_LOG_FILE}" ]]; then
  echo "Log file: ${PF_LOG_FILE}"
fi
