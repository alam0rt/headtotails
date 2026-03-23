#!/usr/bin/env bash
set -euo pipefail

PF_PID_FILE="${PF_PID_FILE:-.kind/control-router.port-forward.pid}"

if [[ ! -f "${PF_PID_FILE}" ]]; then
  echo "No local port-forward pid file found"
  exit 0
fi

pid="$(cat "${PF_PID_FILE}")"
if kill -0 "${pid}" >/dev/null 2>&1; then
  kill "${pid}" || true
  echo "Stopped local port-forward (pid=${pid})"
else
  echo "Port-forward process was not running (pid=${pid})"
fi
rm -f "${PF_PID_FILE}"
