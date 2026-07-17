#!/bin/sh

set -eu

LATEST_VERSION="${1:-${LATEST_VERSION:-}}"
PROJECT_PATH=/opt/sub2-Expansion
UPDATER_NAME=sub2-expansion-updater

if ! printf '%s' "$LATEST_VERSION" | grep -Eq '^v?[0-9]+\.[0-9]+([.][0-9]+)?([+-][0-9A-Za-z.-]+)?$'; then
  echo "Invalid release version: $LATEST_VERSION" >&2
  exit 1
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "Docker CLI is not available in the backend container." >&2
  exit 1
fi

if [ ! -S /var/run/docker.sock ]; then
  echo "Docker socket is not mounted at /var/run/docker.sock." >&2
  exit 1
fi

CONTAINER_ID="$(hostname)"
HOST_PROJECT_DIR="$(docker inspect --format '{{range .Mounts}}{{if eq .Destination "/opt/sub2-Expansion"}}{{.Source}}{{end}}{{end}}' "$CONTAINER_ID")"
UPDATER_IMAGE="$(docker inspect --format '{{.Config.Image}}' "$CONTAINER_ID")"

if [ -z "$HOST_PROJECT_DIR" ]; then
  echo "Cannot determine the host project directory from the backend container mount." >&2
  exit 1
fi

if docker inspect "$UPDATER_NAME" >/dev/null 2>&1; then
  echo "A system update is already running." >&2
  exit 1
fi

mkdir -p "$PROJECT_PATH/logs"

UPDATER_ID="$(docker run --detach --rm \
  --name "$UPDATER_NAME" \
  --volume /var/run/docker.sock:/var/run/docker.sock \
  --volume "$HOST_PROJECT_DIR:$PROJECT_PATH" \
  --env "LATEST_VERSION=$LATEST_VERSION" \
  --env "PROJECT_DIR=$HOST_PROJECT_DIR" \
  --workdir "$PROJECT_PATH" \
  --entrypoint /bin/sh \
  "$UPDATER_IMAGE" \
  -c '
    exec >/opt/sub2-Expansion/logs/system-update.log 2>&1
    set -eu
    git config --global --add safe.directory /opt/sub2-Expansion
    git fetch --tags origin
    git checkout "$LATEST_VERSION"
    if grep -q "^APP_VERSION=" .env; then
      sed -i "s#^APP_VERSION=.*#APP_VERSION=$LATEST_VERSION#" .env
    else
      printf "\nAPP_VERSION=%s\n" "$LATEST_VERSION" >> .env
    fi
    docker compose --project-directory /opt/sub2-Expansion -f /opt/sub2-Expansion/docker-compose.yml up -d --build
  ')"

echo "Update helper started: $UPDATER_ID"
echo "Log: $PROJECT_PATH/logs/system-update.log"
