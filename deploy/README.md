# go-trader deployment

Single-container deployment to the `rileyseaburg` namespace, exposed at
`https://trade.rileyseaburg.com` via the existing cloudflared tunnel.

## Topology

```
trade.rileyseaburg.com
        │  (CNAME → dc7f7221-…cfargotunnel.com, Cloudflare-proxied)
        ▼
cloudflared (cloudflare/cloudflared deployment)
        │  ingress rule: trade.rileyseaburg.com →
        │  http://go-trader.rileyseaburg.svc.cluster.local:8080
        ▼
Service: go-trader (ClusterIP, :8080)
        │
        ▼
Deployment: go-trader (1 replica, Rust Axum, nonroot)
        - args: --port 8080 --mock
        - envFrom: secret/go-trader-secrets (FRED_API_KEY etc.)
        - imagePullSecrets: gcp-artifact-registry
```

The image is built from this repo's root `Dockerfile` (multi-stage:
React→Rust binary) and pushed to GCP Artifact Registry under
`us-central1-docker.pkg.dev/spotlessbinco/rileyseaburg/go-trader`.

## Manifests

| File                  | Resource                                                 |
| --------------------- | -------------------------------------------------------- |
| `namespace.yaml`      | The `rileyseaburg` namespace                             |
| `pull-secret.sh`      | Creates `gcp-artifact-registry` Secret from Vault        |
| `app-secret.sh`       | Creates `go-trader-secrets` from Vault FRED key          |
| `deployment.yaml`     | Deployment + ClusterIP Service                           |
| `tunnel-route.yaml`   | Patch for `cloudflared-config` ConfigMap (one new entry) |
| `build-and-push.sh`   | docker build → tag with git sha → push                   |
| `apply.sh`            | End-to-end deploy: build, push, create secrets, apply    |

## First-time deploy

```bash
cd deploy && ./apply.sh
```

The script is idempotent — re-running rolls a new image and re-applies
manifests. Cloudflare DNS (the `trade.rileyseaburg.com` CNAME) is
created separately; see `apply.sh` for details.