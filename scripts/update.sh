#!/bin/sh

set -eu

LATEST_VERSION="${1:-${LATEST_VERSION:-}}"
PROJECT_PATH=/opt/sub2-Expansion
UPDATER_NAME=sub2-expansion-updater
STATUS_FILE="$PROJECT_PATH/logs/system-update.state"
TASK_ID="${SYSTEM_UPDATE_TASK_ID:-update-$(date -u +%Y%m%dT%H%M%SZ)-$$}"
CURRENT_VERSION="${CURRENT_VERSION:-}"

write_state() {
  status="$1"
  message="$2"
  finished_at="${3:-}"
  mkdir -p "$PROJECT_PATH/logs"
  temp_file="$STATUS_FILE.$$.tmp"
  {
    printf 'task_id=%s\n' "$TASK_ID"
    printf 'status=%s\n' "$status"
    printf 'current_version=%s\n' "$CURRENT_VERSION"
    printf 'target_version=%s\n' "$LATEST_VERSION"
    printf 'started_at=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    printf 'finished_at=%s\n' "$finished_at"
    printf 'message=%s\n' "$message"
  } > "$temp_file"
  mv "$temp_file" "$STATUS_FILE"
}

fail_start() {
  write_state FAILED "$1" "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo "$1" >&2
  exit 1
}

if ! printf '%s' "$LATEST_VERSION" | grep -Eq '^v?[0-9]+\.[0-9]+([.][0-9]+)?([+-][0-9A-Za-z.-]+)?$'; then
  fail_start "Invalid release version: $LATEST_VERSION"
fi

if ! command -v docker >/dev/null 2>&1; then
  fail_start "Docker CLI is not available in the backend container."
fi

if [ ! -S /var/run/docker.sock ]; then
  fail_start "Docker socket is not mounted at /var/run/docker.sock."
fi

CONTAINER_ID="$(hostname)"
HOST_PROJECT_DIR="$(docker inspect --format '{{range .Mounts}}{{if eq .Destination "/opt/sub2-Expansion"}}{{.Source}}{{end}}{{end}}' "$CONTAINER_ID")"
UPDATER_IMAGE="$(docker inspect --format '{{.Config.Image}}' "$CONTAINER_ID")"

if [ -z "$HOST_PROJECT_DIR" ]; then
  fail_start "Cannot determine the host project directory from the backend container mount."
fi

if docker inspect "$UPDATER_NAME" >/dev/null 2>&1; then
  fail_start "A system update is already running."
fi

write_state STARTING "Creating immutable-image update worker."

if ! UPDATER_ID="$(docker run --detach --rm \
  --name "$UPDATER_NAME" \
  --volume /var/run/docker.sock:/var/run/docker.sock \
  --volume "$HOST_PROJECT_DIR:$PROJECT_PATH" \
  --env "LATEST_VERSION=$LATEST_VERSION" \
  --env "CURRENT_VERSION=$CURRENT_VERSION" \
  --env "SYSTEM_UPDATE_TASK_ID=$TASK_ID" \
  --env "PROJECT_DIR=$HOST_PROJECT_DIR" \
  --workdir "$PROJECT_PATH" \
  --entrypoint /bin/sh \
  "$UPDATER_IMAGE" \
  -c '
    set -eu
    PROJECT_PATH=/opt/sub2-Expansion
    STATUS_FILE="$PROJECT_PATH/logs/system-update.state"
    TASK_ID="$SYSTEM_UPDATE_TASK_ID"
    started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

    write_state() {
      status="$1"
      message="$2"
      finished_at="${3:-}"
      temp_file="$STATUS_FILE.$$.tmp"
      {
        printf "task_id=%s\\n" "$TASK_ID"
        printf "status=%s\\n" "$status"
        printf "current_version=%s\\n" "$CURRENT_VERSION"
        printf "target_version=%s\\n" "$LATEST_VERSION"
        printf "started_at=%s\\n" "$started_at"
        printf "finished_at=%s\\n" "$finished_at"
        printf "message=%s\\n" "$message"
      } > "$temp_file"
      mv "$temp_file" "$STATUS_FILE"
    }

    on_exit() {
      exit_code=$?
      if [ "$exit_code" -ne 0 ]; then
        write_state FAILED "Update failed. Review the task log." "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
      fi
    }

    mkdir -p "$PROJECT_PATH/logs"
    exec >"$PROJECT_PATH/logs/system-update.log" 2>&1
    trap on_exit EXIT
    write_state RUNNING "Pulling release images."
    printf "Updating from %s to %s\\n" "$CURRENT_VERSION" "$LATEST_VERSION"

    if [ ! -f .env ]; then
      echo "Missing .env in $PROJECT_PATH" >&2
      exit 1
    fi

    if grep -q "^APP_VERSION=" .env; then
      sed -i "s#^APP_VERSION=.*#APP_VERSION=$LATEST_VERSION#" .env
    else
      printf "\\nAPP_VERSION=%s\\n" "$LATEST_VERSION" >> .env
    fi

    docker compose --project-directory "$PROJECT_PATH" -f "$PROJECT_PATH/docker-compose.yml" pull backend frontend
    write_state RUNNING "Recreating services from release images."
    docker compose --project-directory "$PROJECT_PATH" -f "$PROJECT_PATH/docker-compose.yml" up -d --no-build backend frontend

    for service in backend frontend; do
      container_id="$(docker compose --project-directory "$PROJECT_PATH" -f "$PROJECT_PATH/docker-compose.yml" ps -q "$service")"
      if [ -z "$container_id" ]; then
        echo "Service $service was not created" >&2
        exit 1
      fi
      healthy=false
      for attempt in $(seq 1 60); do
        health="$(docker inspect --format "{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}" "$container_id")"
        if [ "$health" = healthy ] || [ "$health" = running ]; then
          healthy=true
          break
        fi
        sleep 2
      done
      if [ "$healthy" != true ]; then
        echo "Service $service did not become healthy within 120 seconds" >&2
        exit 1
      fi
    done

    write_state SUCCEEDED "Release images are healthy." "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    printf "Update completed successfully.\\n"
    trap - EXIT
  ')"; then
  write_state FAILED "Could not start the immutable-image update worker." "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  exit 1
fi

echo "Update helper started: $UPDATER_ID"
echo "Task: $TASK_ID"
echo "Log: $PROJECT_PATH/logs/system-update.log"
