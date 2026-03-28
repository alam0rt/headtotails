#!/usr/bin/env bash
# 07-acl-policy.sh — read and write ACL policy via the headtotails API.
#
# Exercises:
#   POST /api/v2/oauth/token       (obtain bearer token)
#   GET  /api/v2/tailnet/-/acl     (read current ACL policy)
#   POST /api/v2/tailnet/-/acl     (write new ACL policy)
#
# Usage:
#   export HEADTOTAILS_URL="https://hs.example.com"
#   export OAUTH_CLIENT_ID="..."
#   export OAUTH_CLIENT_SECRET="..."
#   ./07-acl-policy.sh
set -euo pipefail

: "${HEADTOTAILS_URL:?Set HEADTOTAILS_URL to your headtotails base URL}"
: "${OAUTH_CLIENT_ID:?Set OAUTH_CLIENT_ID}"
: "${OAUTH_CLIENT_SECRET:?Set OAUTH_CLIENT_SECRET}"

echo "==> Obtaining OAuth token..."
TOKEN_RESPONSE=$(curl -fsSL -X POST "${HEADTOTAILS_URL}/api/v2/oauth/token" \
  -u "${OAUTH_CLIENT_ID}:${OAUTH_CLIENT_SECRET}" \
  -d "grant_type=client_credentials")

ACCESS_TOKEN=$(echo "${TOKEN_RESPONSE}" | jq -r '.access_token')
if [[ -z "${ACCESS_TOKEN}" || "${ACCESS_TOKEN}" == "null" ]]; then
  echo "ERROR: failed to obtain access token" >&2
  echo "${TOKEN_RESPONSE}" >&2
  exit 1
fi
echo "    token acquired (${#ACCESS_TOKEN} chars)"

echo "==> Reading current ACL policy..."
curl -fsSL -X GET "${HEADTOTAILS_URL}/api/v2/tailnet/-/acl" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" | jq .
echo ""

echo "==> Writing new ACL policy (covers all example tags)..."
# HuJSON policy that grants tag ownership for every tag used across the
# example manifests (tag:k8s, tag:demo, tag:subnet, tag:exit).
POLICY=$(cat <<'EOF'
{
  "tagOwners": {
    "tag:k8s":    ["autogroup:admin"],
    "tag:demo":   ["autogroup:admin"],
    "tag:subnet": ["autogroup:admin"],
    "tag:exit":   ["autogroup:admin"]
  },
  "acls": [
    {
      "action": "accept",
      "src":    ["tag:k8s"],
      "dst":    ["*:*"]
    },
    {
      "action": "accept",
      "src":    ["autogroup:member"],
      "dst":    ["tag:demo:*"]
    }
  ]
}
EOF
)

curl -fsSL -X POST "${HEADTOTAILS_URL}/api/v2/tailnet/-/acl" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "Content-Type: application/json" \
  -d "${POLICY}" | jq .

echo ""
echo "==> Done. ACL policy updated."
