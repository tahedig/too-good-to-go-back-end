#!/usr/bin/env bash

set -Eeuo pipefail

readonly api_container="staging-backend-api"
readonly db_container="staging-backend-db"
readonly compose_file="compose.yaml"
readonly candidate_file="compose.yaml.candidate"
readonly env_file=".env"
readonly env_backup=".env.rollback"
readonly compose_backup="compose.yaml.rollback"
readonly pending_image_file=".pending-image"
readonly pending_sha_file=".pending-sha"
readonly backup_dir="backups"
readonly app_network="staging-app"

exec 9> .deploy.lock
if ! flock -n 9; then
  echo "Another backend deployment is already running" >&2
  exit 1
fi

wait_for_health() {
  local container="$1"
  local status

  for _ in {1..45}; do
    status="$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "$container" 2>/dev/null || true)"
    if [[ "$status" == "healthy" ]]; then
      return 0
    fi
    if [[ "$status" == "unhealthy" ]]; then
      docker logs --tail 100 "$container" >&2 || true
      return 1
    fi
    sleep 2
  done

  docker logs --tail 100 "$container" >&2 || true
  echo "Timed out waiting for $container to become healthy" >&2
  return 1
}

restore_previous_api() {
  set +e

  if [[ -f "$env_backup" ]]; then
    mv -f "$env_backup" "$env_file"
  fi
  if [[ -f "$compose_backup" ]]; then
    mv -f "$compose_backup" "$compose_file"
  fi

  if [[ -f "$env_file" && -f "$compose_file" ]]; then
    docker compose --env-file "$env_file" -f "$compose_file" up \
      --detach --no-build --force-recreate api
    wait_for_health "$api_container"
  else
    docker rm --force "$api_container" >/dev/null 2>&1 || true
  fi

  rm -f "$candidate_file" "$pending_image_file" "$pending_sha_file"
}

update_image() {
  local image="$1"
  local temp_file
  temp_file="$(mktemp .env.XXXXXX)"

  awk -v image="$image" '
    BEGIN { replaced = 0 }
    /^BACKEND_IMAGE=/ {
      print "BACKEND_IMAGE=" image
      replaced = 1
      next
    }
    { print }
    END {
      if (!replaced) {
        print "BACKEND_IMAGE=" image
      }
    }
  ' "$env_file" > "$temp_file"

  chmod 600 "$temp_file"
  mv -f "$temp_file" "$env_file"
}

create_database_backup() {
  local backup_file

  install -d -m 700 "$backup_dir"
  backup_file="$backup_dir/pre-deploy-$(date -u +%Y%m%dT%H%M%SZ).dump"

  docker exec "$db_container" sh -ec \
    'exec pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --format=custom' \
    > "$backup_file"

  if [[ ! -s "$backup_file" ]]; then
    rm -f "$backup_file"
    echo "Pre-deployment database backup is empty" >&2
    return 1
  fi

  chmod 600 "$backup_file"
  mapfile -t old_backups < <(find "$backup_dir" -maxdepth 1 -type f -name 'pre-deploy-*.dump' -printf '%T@ %p\n' | sort -rn | tail -n +6 | cut -d' ' -f2-)
  if (( ${#old_backups[@]} > 0 )); then
    rm -f -- "${old_backups[@]}"
  fi

  echo "Database backup created: $backup_file"
}

run_migrations() {
  local image="$1"

  docker run --rm \
    --network "$app_network" \
    --env-file "$env_file" \
    --entrypoint /bin/sh \
    "$image" \
    -ec 'exec migrate -path /migrations -database "$DATABASE_URL" up'
}

deploy() {
  local new_image="${1:?New image is required}"
  local expected_sha="${2:?Expected commit SHA is required}"
  local image_sha
  local health_json

  if [[ ! "$expected_sha" =~ ^[0-9a-f]{40}$ ]]; then
    echo "Expected SHA must be a full 40-character Git commit SHA" >&2
    return 1
  fi
  if [[ "$new_image" != *":develop-$expected_sha" ]]; then
    echo "Image tag does not match the expected commit SHA" >&2
    return 1
  fi
  if [[ ! -f "$env_file" ]]; then
    echo "$env_file is missing on the staging server" >&2
    return 1
  fi
  if [[ ! -f "$candidate_file" ]]; then
    echo "$candidate_file is missing" >&2
    return 1
  fi
  if [[ -e "$env_backup" || -e "$compose_backup" ]]; then
    echo "An earlier backend deployment is still pending confirmation or rollback" >&2
    return 1
  fi

  docker compose --env-file "$env_file" -f "$candidate_file" config >/dev/null
  docker pull "$new_image"

  image_sha="$(docker image inspect "$new_image" --format '{{ index .Config.Labels "org.opencontainers.image.revision" }}')"
  if [[ "$image_sha" != "$expected_sha" ]]; then
    echo "Image revision $image_sha does not match expected commit $expected_sha" >&2
    return 1
  fi

  cp -p "$env_file" "$env_backup"
  if [[ -f "$compose_file" ]]; then
    cp -p "$compose_file" "$compose_backup"
  fi

  trap 'status=$?; trap - ERR; echo "Backend deployment failed; restoring the previous API image" >&2; restore_previous_api; exit "$status"' ERR

  docker compose --env-file "$env_file" -f "$candidate_file" up --detach db redis
  wait_for_health "$db_container"
  wait_for_health "staging-backend-redis"

  create_database_backup
  run_migrations "$new_image"

  mv -f "$candidate_file" "$compose_file"
  update_image "$new_image"

  docker compose --env-file "$env_file" -f "$compose_file" up \
    --detach --no-build --force-recreate api
  wait_for_health "$api_container"

  health_json="$(docker exec "$api_container" wget --quiet -O - http://127.0.0.1:8080/health)"
  if [[ "$health_json" != *'"status":"ok"'* || "$health_json" != *"\"version\":\"$expected_sha\""* ]]; then
    echo "API health response does not identify healthy commit $expected_sha: $health_json" >&2
    return 1
  fi

  printf '%s\n' "$new_image" > "$pending_image_file"
  printf '%s\n' "$expected_sha" > "$pending_sha_file"
  trap - ERR

  echo "Backend deployment is healthy and awaiting host confirmation"
}

confirm() {
  if [[ ! -s "$pending_image_file" || ! -s "$pending_sha_file" ]]; then
    echo "There is no pending backend deployment to confirm" >&2
    return 1
  fi

  if [[ -f "$env_backup" ]]; then
    awk -F= '/^BACKEND_IMAGE=/ { print $2 }' "$env_backup" > .previous-healthy-image
  fi
  cp "$pending_image_file" .last-healthy-image
  cp "$pending_sha_file" .last-healthy-sha

  rm -f "$env_backup" "$compose_backup" "$pending_image_file" "$pending_sha_file"
  echo "Backend deployment confirmed as healthy"
}

rollback() {
  if [[ ! -f "$env_backup" ]]; then
    echo "There is no previous backend deployment state to restore" >&2
    return 1
  fi

  restore_previous_api
  echo "Previous backend API image restored; database migrations were not reversed"
}

case "${1:-}" in
  deploy)
    shift
    deploy "$@"
    ;;
  confirm)
    confirm
    ;;
  rollback)
    rollback
    ;;
  *)
    echo "Usage: $0 {deploy <image> <commit-sha>|confirm|rollback}" >&2
    exit 2
    ;;
esac
