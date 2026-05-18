#!/bin/bash
# Add (or replace) the trade.rileyseaburg.com route in the cloudflared
# tunnel configuration.
#
# IMPORTANT: this cluster's tunnel is **remotely-managed** by Cloudflare
# Zero Trust (cloudflared loads ingress rules from the API, not from the
# in-cluster cloudflared-config ConfigMap). Editing the ConfigMap was a
# no-op until this script was rewritten to call the Cloudflare API.
#
# Reads the Cloudflare token + account_id from Vault at
# kv/cloudflare/api-token (fields: token, account_id). Tunnel ID is the
# canonical cluster tunnel pulled from the existing cloudflared-config
# ConfigMap as a sanity reference.

set -euo pipefail

TUNNEL_HOSTNAME="${TUNNEL_HOSTNAME:-trade.rileyseaburg.com}"
SERVICE="${SERVICE:-http://go-trader.rileyseaburg.svc.cluster.local:8080}"
TUNNEL_ID="${TUNNEL_ID:-dc7f7221-95ad-4cfb-b679-b3473cef4f1c}"

CF_TOKEN="$(vault kv get -field=token kv/cloudflare/api-token)"
ACCOUNT_ID="$(vault kv get -field=account_id kv/cloudflare/api-token)"

API="https://api.cloudflare.com/client/v4/accounts/${ACCOUNT_ID}/cfd_tunnel/${TUNNEL_ID}/configurations"

cf_request() {
  local method="$1"
  local url="$2"
  local body="${3:-}"
  local resp_file status
  resp_file="$(mktemp)"

  if [[ -n "${body}" ]]; then
    status="$(curl -sS -o "${resp_file}" -w "%{http_code}" -X "${method}" \
      -H "Authorization: Bearer ${CF_TOKEN}" \
      -H "Content-Type: application/json" \
      "${url}" --data "${body}" || true)"
  else
    status="$(curl -sS -o "${resp_file}" -w "%{http_code}" -X "${method}" \
      -H "Authorization: Bearer ${CF_TOKEN}" \
      "${url}" || true)"
  fi

  if [[ ! "${status}" =~ ^2[0-9][0-9]$ ]]; then
    echo "Cloudflare API ${method} ${url} failed with HTTP ${status}" >&2
    echo "Raw response body:" >&2
    sed 's/^/  /' "${resp_file}" >&2 || true
    rm -f "${resp_file}"
    exit 1
  fi

  cat "${resp_file}"
  rm -f "${resp_file}"
}

CURRENT_JSON="$(cf_request GET "${API}")"

NEW_PAYLOAD="$(echo "${CURRENT_JSON}" | python3 - "${TUNNEL_HOSTNAME}" "${SERVICE}" <<'PY'
import sys, json
hostname, service = sys.argv[1], sys.argv[2]
raw = sys.stdin.read()
try:
    d = json.loads(raw)
except json.JSONDecodeError as e:
    print(f"Cloudflare GET returned invalid JSON: {e}", file=sys.stderr)
    print("Raw response body:", file=sys.stderr)
    print(raw, file=sys.stderr)
    sys.exit(1)
if not d.get('success', False):
    print("Cloudflare GET returned success=false:", file=sys.stderr)
    print(json.dumps(d, indent=2), file=sys.stderr)
    sys.exit(1)
result = d.get('result') or {}
cfg = result.get('config')
if not isinstance(cfg, dict):
    print("Cloudflare GET response missing result.config object:", file=sys.stderr)
    print(json.dumps(d, indent=2), file=sys.stderr)
    sys.exit(1)
ingress = cfg.get('ingress', [])
if not isinstance(ingress, list):
    print("Cloudflare tunnel config ingress is not a list", file=sys.stderr)
    sys.exit(1)
ingress = [r for r in ingress if r.get('hostname') != hostname]
new_rule = {'hostname': hostname, 'service': service}
out, inserted = [], False
for r in ingress:
    if not inserted and 'hostname' not in r:
        out.append(new_rule); inserted = True
    out.append(r)
if not inserted: out.append(new_rule)
cfg['ingress'] = out
print(json.dumps({'config': cfg}))
PY
)"

RESP="$(cf_request PUT "${API}" "${NEW_PAYLOAD}")"

OK=$(echo "${RESP}" | python3 - <<'PY'
import json, sys
raw = sys.stdin.read()
try:
    print(json.loads(raw).get("success", False))
except json.JSONDecodeError as e:
    print(f"Cloudflare PUT returned invalid JSON: {e}", file=sys.stderr)
    print("Raw response body:", file=sys.stderr)
    print(raw, file=sys.stderr)
    sys.exit(2)
PY
)
if [[ "${OK}" != "True" ]]; then
  echo "tunnel config update failed:" >&2
  echo "${RESP}" >&2
  exit 1
fi
echo "✓ tunnel route added: ${TUNNEL_HOSTNAME} → ${SERVICE} (Cloudflare-managed config)"