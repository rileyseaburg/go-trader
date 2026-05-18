#!/bin/bash
# End-to-end deploy of go-trader to the rileyseaburg namespace.
#
#   1. apply Namespace
#   2. write pull secret + app secret from Vault
#   3. build image, push to GAR, capture image ref
#   4. apply Deployment + Service with the new image ref
#   5. wait for rollout
#   6. add tunnel route in cloudflared-config + restart cloudflared
#   7. ensure DNS CNAME exists in Cloudflare
#   8. probe the public hostname
#
# Idempotent — safe to re-run after code changes; will roll a new image.

set -euo pipefail
cd "$(dirname "$0")"

NAMESPACE="${NAMESPACE:-rileyseaburg}"
# Using PUBLIC_HOSTNAME, not HOSTNAME — bash auto-sets $HOSTNAME.
PUBLIC_HOSTNAME="${PUBLIC_HOSTNAME:-trade.rileyseaburg.com}"

step() { echo; echo "── $* ──"; }

step "1/8  namespace"
kubectl apply -f namespace.yaml

step "2/8  secrets"
./pull-secret.sh
./app-secret.sh

step "3/8  build + push image"
IMAGE="$(./build-and-push.sh)"
echo "image: ${IMAGE}"

step "4/8  patch Deployment + apply manifests"
# kubectl set image is the cleanest in-place patch. We still apply the
# Deployment manifest first so any spec changes (resources, args, etc.)
# come through.
kubectl apply -f deployment.yaml
kubectl -n "${NAMESPACE}" set image deployment/go-trader "trader=${IMAGE}"

step "5/8  wait for rollout"
kubectl -n "${NAMESPACE}" rollout status deployment/go-trader --timeout=180s

step "6/8  tunnel route"
./tunnel-route.sh

step "7/8  DNS"
./dns.sh

step "8/8  probe ${PUBLIC_HOSTNAME}"
# Cloudflare DNS may take 30-60s to propagate; retry a few times.
for i in 1 2 3 4 5 6; do
  CODE=$(curl -sS -o /dev/null -w "%{http_code}" "https://${PUBLIC_HOSTNAME}/api/cartography" || echo 000)
  if [[ "${CODE}" == "200" ]]; then
    echo "✓ ${PUBLIC_HOSTNAME}/api/cartography → 200"
    curl -sS "https://${PUBLIC_HOSTNAME}/api/cartography" | python3 -c "
import sys, json
d = json.load(sys.stdin)
print(f\"  regime: {d['reading']['regime']['name']} ×{d['reading']['regime']['multiplier']:.2f}\")
print(f\"  data_integration: {d.get('data_integration')}\")
"
    exit 0
  fi
  echo "  attempt ${i}: HTTP ${CODE}, waiting 10s…"
  sleep 10
done

echo "✗ ${PUBLIC_HOSTNAME} not reachable after 60s — check cloudflared logs:" >&2
echo "    kubectl -n cloudflare logs -l app=cloudflared --tail=50" >&2
exit 1