#!/bin/bash
# Build the multi-stage Dockerfile, tag it with the current git short
# SHA, and push to GCP Artifact Registry. Echoes the full image ref so
# callers can pipe it into a deployment patch.

set -euo pipefail

REGISTRY="${REGISTRY:-us-central1-docker.pkg.dev}"
PROJECT="${PROJECT:-spotlessbinco}"
# Reuse the existing spotlessbinco repo (same one as voice-agent etc.)
# rather than creating a new GAR repo for this single image. The
# repo/app convention here is <gcp-project>/<repo>/<image>:<tag>.
REPO="${REPO:-spotlessbinco}"
APP="${APP:-go-trader}"

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SHA="$(git -C "${REPO_ROOT}" rev-parse --short HEAD)"
ENV="${ENV:-paper}"
if ! git -C "${REPO_ROOT}" diff-index --quiet HEAD --; then
  echo "ERROR: working tree is dirty. Commit or stash before building." >&2
  echo "Use SKIP_CLEAN_CHECK=1 to override (not recommended)." >&2
  [ "${SKIP_CLEAN_CHECK:-}" = "1" ] || exit 1
fi
TAG="${SHA}"
IMAGE="${REGISTRY}/${PROJECT}/${REPO}/${APP}:${TAG}"
LATEST_IMAGE="${REGISTRY}/${PROJECT}/${REPO}/${APP}:latest"

echo ">> building ${IMAGE}" >&2
docker build -t "${IMAGE}" -t "${LATEST_IMAGE}" "${REPO_ROOT}" >&2

# Auth to GAR using the service-account key from Vault, in an isolated
# DOCKER_CONFIG dir so we never touch ~/.docker/config.json (which uses
# the gcloud credential helper and may have expired tokens). The temp
# config is wiped on exit.
TMP_DOCKER_CFG="$(mktemp -d)"
trap 'rm -rf "${TMP_DOCKER_CFG}"' EXIT

KEY_JSON="$(vault kv get -field=key_json secret/ci/gcp-artifact-registry)"
echo "${KEY_JSON}" | DOCKER_CONFIG="${TMP_DOCKER_CFG}" \
  docker login -u _json_key --password-stdin "https://${REGISTRY}" >&2

echo ">> pushing ${IMAGE}" >&2
DOCKER_CONFIG="${TMP_DOCKER_CFG}" docker push "${IMAGE}" >&2
echo ">> pushing ${LATEST_IMAGE}" >&2
DOCKER_CONFIG="${TMP_DOCKER_CFG}" docker push "${LATEST_IMAGE}" >&2

DEPLOY_TAG="deploy/$(date -u +%Y%m%d-%H%M%S)-${SHA}"
git -C "${REPO_ROOT}" tag -a "${DEPLOY_TAG}" -m "Deployed to ${ENV}"
echo ">> tagged git ${DEPLOY_TAG}" >&2

# Print only the immutable image ref on stdout so the caller can capture it.
echo "${IMAGE}"
