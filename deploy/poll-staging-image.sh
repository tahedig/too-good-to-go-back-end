#!/usr/bin/env bash

set -Eeuo pipefail

readonly registry_image="${BACKEND_REGISTRY_IMAGE:-ghcr.io/tahedig/too-good-to-go-back-end}"
readonly tracking_image="${registry_image}:main"
readonly deploy_dir="${BACKEND_DEPLOY_DIR:-/home/fpna/apps/backend}"
readonly health_url="${STAGING_BACKEND_HEALTH_URL:-http://127.0.0.1:8081}"
readonly container_name="staging-backend-api"

cd "$deploy_dir"

exec 8> .poll.lock
if ! flock -n 8; then
  echo "Another backend image check is already running"
  exit 0
fi

for required_file in .env compose.template.yaml deploy-staging.sh; do
  if [[ ! -f "$required_file" ]]; then
    echo "$deploy_dir/$required_file is missing" >&2
    exit 1
  fi
done

echo "Checking $tracking_image for a new staging image"
docker pull "$tracking_image"

candidate_sha="$(docker image inspect "$tracking_image" \
  --format '{{ index .Config.Labels "org.opencontainers.image.revision" }}')"

if [[ ! "$candidate_sha" =~ ^[0-9a-f]{40}$ ]]; then
  echo "$tracking_image does not contain a valid commit revision label" >&2
  exit 1
fi

candidate_image="${registry_image}:main-${candidate_sha}"
docker pull "$candidate_image"

tracking_id="$(docker image inspect "$tracking_image" --format '{{.Id}}')"
candidate_id="$(docker image inspect "$candidate_image" --format '{{.Id}}')"
if [[ "$tracking_id" != "$candidate_id" ]]; then
  echo "Mutable and immutable backend tags do not identify the same image" >&2
  exit 1
fi

current_sha="$(docker inspect "$container_name" \
  --format '{{ index .Config.Labels "org.opencontainers.image.revision" }}' \
  2>/dev/null || true)"

if [[ "$current_sha" == "$candidate_sha" ]]; then
  printf '%s\n' "$candidate_image" > .last-healthy-image
  printf '%s\n' "$candidate_sha" > .last-healthy-sha
  echo "Staging backend already runs commit $candidate_sha"
  exit 0
fi

cp -f compose.template.yaml compose.yaml.candidate
chmod 600 compose.yaml.candidate

deployment_started=false
rollback_if_needed() {
  local status=$?
  trap - ERR

  if [[ "$deployment_started" == true ]]; then
    echo "Backend host verification failed; restoring the previous API image" >&2
    ./deploy-staging.sh rollback || true
  fi

  exit "$status"
}
trap rollback_if_needed ERR

./deploy-staging.sh deploy "$candidate_image" "$candidate_sha"
deployment_started=true

base_url="${health_url%/}"
health_json="$(curl --fail --silent --show-error \
  --connect-timeout 5 --max-time 15 "$base_url/health")"

if [[ "$health_json" != *'"status":"ok"'* || "$health_json" != *"\"version\":\"$candidate_sha\""* ]]; then
  echo "Staging backend health does not match $candidate_sha: $health_json" >&2
  false
fi

./deploy-staging.sh confirm
deployment_started=false
trap - ERR

echo "Staging backend now runs $candidate_image"
