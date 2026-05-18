#!/bin/bash
# Create (or update) the trade.rileyseaburg.com CNAME record in
# Cloudflare DNS pointing to the cloudflared tunnel hostname. The record
# is proxied (orange-cloud) so Cloudflare terminates TLS and routes
# through the tunnel.

set -euo pipefail

ZONE_NAME="${ZONE_NAME:-rileyseaburg.com}"
RECORD_NAME="${RECORD_NAME:-trade.rileyseaburg.com}"
TUNNEL_ID="${TUNNEL_ID:-dc7f7221-95ad-4cfb-b679-b3473cef4f1c}"
CNAME_TARGET="${TUNNEL_ID}.cfargotunnel.com"

CF_TOKEN="$(vault kv get -field=token kv/cloudflare/api-token)"
if [[ -z "${CF_TOKEN}" ]]; then
  echo "Vault did not return a Cloudflare token" >&2
  exit 1
fi

api() {
  curl -sS -H "Authorization: Bearer ${CF_TOKEN}" \
    -H "Content-Type: application/json" "$@"
}

ZONE_ID=$(api "https://api.cloudflare.com/client/v4/zones?name=${ZONE_NAME}" \
  | python3 -c 'import sys,json; r=json.load(sys.stdin)["result"]; print(r[0]["id"] if r else "")')
if [[ -z "${ZONE_ID}" ]]; then
  echo "zone ${ZONE_NAME} not found in Cloudflare" >&2
  exit 1
fi

# Look up an existing record at the target name. If it exists, PUT to
# update; otherwise POST to create.
EXISTING_ID=$(api "https://api.cloudflare.com/client/v4/zones/${ZONE_ID}/dns_records?name=${RECORD_NAME}&type=CNAME" \
  | python3 -c 'import sys,json; r=json.load(sys.stdin)["result"]; print(r[0]["id"] if r else "")')

PAYLOAD=$(python3 -c "import json; print(json.dumps({'type':'CNAME','name':'${RECORD_NAME}','content':'${CNAME_TARGET}','proxied':True,'ttl':1,'comment':'go-trader (auto)'}))")

if [[ -z "${EXISTING_ID}" ]]; then
  RESP=$(api -X POST "https://api.cloudflare.com/client/v4/zones/${ZONE_ID}/dns_records" \
    --data "${PAYLOAD}")
  ACTION=created
else
  RESP=$(api -X PUT "https://api.cloudflare.com/client/v4/zones/${ZONE_ID}/dns_records/${EXISTING_ID}" \
    --data "${PAYLOAD}")
  ACTION=updated
fi

OK=$(echo "${RESP}" | python3 -c 'import sys,json; print(json.load(sys.stdin)["success"])' 2>/dev/null || echo False)
if [[ "${OK}" != "True" ]]; then
  echo "DNS ${ACTION} failed:" >&2
  echo "${RESP}" >&2
  exit 1
fi
echo "✓ DNS ${ACTION}: ${RECORD_NAME} → ${CNAME_TARGET} (proxied)"
