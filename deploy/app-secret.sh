#!/bin/bash
# Create the go-trader-secrets Secret in the rileyseaburg namespace,
# sourcing app credentials from Vault. Pod consumes via envFrom.
#
# Why bake Vault values into a k8s Secret rather than have the pod read
# Vault at startup: the pod does not need cluster-internal Vault access,
# secret values never land in env locally during build, and rotation is
# one `vault kv put` + this script away.

set -euo pipefail

NAMESPACE="${NAMESPACE:-rileyseaburg}"
FRED_VAULT_PATH="${FRED_VAULT_PATH:-secret/go-trader/fred}"
FRED_VAULT_FIELD="${FRED_VAULT_FIELD:-api_key}"
ALPACA_VAULT_PATH="${ALPACA_VAULT_PATH:-secret/go-trader/alpaca-paper}"
ALPACA_KEY_FIELD="${ALPACA_KEY_FIELD:-api_key}"
ALPACA_SECRET_FIELD="${ALPACA_SECRET_FIELD:-secret_key}"

FRED_KEY="$(vault kv get -field="${FRED_VAULT_FIELD}" "${FRED_VAULT_PATH}")"
if [[ -z "${FRED_KEY}" ]]; then
  echo "Vault path ${FRED_VAULT_PATH} field ${FRED_VAULT_FIELD} returned empty" >&2
  exit 1
fi

PAPER_ALPACA_API_KEY="$(vault kv get -field="${ALPACA_KEY_FIELD}" "${ALPACA_VAULT_PATH}")"
if [[ -z "${PAPER_ALPACA_API_KEY}" ]]; then
  echo "Vault path ${ALPACA_VAULT_PATH} field ${ALPACA_KEY_FIELD} returned empty" >&2
  exit 1
fi

PAPER_ALPACA_SECRET_KEY="$(vault kv get -field="${ALPACA_SECRET_FIELD}" "${ALPACA_VAULT_PATH}" 2>/dev/null || true)"
if [[ -z "${PAPER_ALPACA_SECRET_KEY}" ]]; then
  echo "Vault path ${ALPACA_VAULT_PATH} field ${ALPACA_SECRET_FIELD} returned empty" >&2
  echo "Add it with: vault kv patch ${ALPACA_VAULT_PATH} ${ALPACA_SECRET_FIELD}=..." >&2
  exit 1
fi

kubectl create secret generic go-trader-secrets \
  --namespace="${NAMESPACE}" \
  --from-literal=FRED_API_KEY="${FRED_KEY}" \
  --from-literal=PAPER_ALPACA_API_KEY="${PAPER_ALPACA_API_KEY}" \
  --from-literal=PAPER_ALPACA_SECRET_KEY="${PAPER_ALPACA_SECRET_KEY}" \
  --dry-run=client -o yaml | kubectl apply -f -

echo "✓ app secret go-trader-secrets written to ${NAMESPACE}"
