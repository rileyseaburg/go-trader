#!/bin/bash
# Create the gcp-artifact-registry pull secret in the rileyseaburg
# namespace, sourcing the GCP service-account key from Vault. Idempotent:
# replaces an existing secret in place rather than failing.

set -euo pipefail

NAMESPACE="${NAMESPACE:-rileyseaburg}"
VAULT_PATH="${VAULT_PATH:-secret/ci/gcp-artifact-registry}"
VAULT_FIELD="${VAULT_FIELD:-key_json}"
REGISTRY="${REGISTRY:-us-central1-docker.pkg.dev}"

KEY_JSON="$(vault kv get -field="${VAULT_FIELD}" "${VAULT_PATH}")"
if [[ -z "${KEY_JSON}" ]]; then
  echo "Vault path ${VAULT_PATH} field ${VAULT_FIELD} returned empty" >&2
  exit 1
fi

# Build a dockerconfigjson with _json_key auth (the GCP service-account
# pattern this cluster already uses across namespaces).
AUTH=$(printf '_json_key:%s' "${KEY_JSON}" | base64 -w0)
DCJ=$(jq -n --arg r "${REGISTRY}" --arg u "_json_key" --arg p "${KEY_JSON}" --arg a "${AUTH}" \
  '{auths: {($r): {username: $u, password: $p, auth: $a}}}')

# Apply (replace) the secret. Don't use `kubectl create` — that fails on
# re-run if the secret exists.
kubectl create secret generic gcp-artifact-registry \
  --namespace="${NAMESPACE}" \
  --type=kubernetes.io/dockerconfigjson \
  --from-literal=.dockerconfigjson="${DCJ}" \
  --dry-run=client -o yaml | kubectl apply -f -

echo "✓ pull secret gcp-artifact-registry written to ${NAMESPACE}"
