#!/usr/bin/env sh
set -eu

REPO_URL="${REPO_URL:-https://github.com/hepingan11/sub2-Expansion.git}"
BRANCH="${BRANCH:-main}"
HTTP_PORT="${HTTP_PORT:-6779}"
ADMIN_USERNAME="${ADMIN_USERNAME:-admin}"
DEFAULT_UPDATE_COMMAND='git config --global --add safe.directory /opt/sub2-Expansion && cd /opt/sub2-Expansion && git fetch --tags origin && git checkout "$LATEST_VERSION" && if grep -q "^APP_VERSION=" .env; then sed -i "s#^APP_VERSION=.*#APP_VERSION=$LATEST_VERSION#" .env; else printf "\nAPP_VERSION=%s\n" "$LATEST_VERSION" >> .env; fi && nohup sh -c "docker compose --project-directory /opt/sub2-Expansion -f /opt/sub2-Expansion/docker-compose.yml up -d --build" >/tmp/sub2-expansion-update.log 2>&1 &'

if [ -n "${INSTALL_DIR:-}" ]; then
  INSTALL_DIR="$INSTALL_DIR"
elif [ -w "." ]; then
  INSTALL_DIR="sub2-Expansion"
else
  INSTALL_DIR="${HOME:-/tmp}/sub2-Expansion"
  echo "Current directory is not writable, using ${INSTALL_DIR}."
fi

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Missing required command: $1" >&2
    exit 1
  fi
}

random_hex() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex "$1"
    return
  fi
  date +%s%N | sha256sum | awk '{print $1}'
}

env_value() {
  key="$1"
  fallback="$2"
  if [ ! -f .env ]; then
    printf '%s\n' "$fallback"
    return
  fi
  value="$(grep -E "^${key}=" .env 2>/dev/null | tail -n 1 | cut -d= -f2- || true)"
  if [ -n "$value" ]; then
    printf '%s\n' "$value"
  else
    printf '%s\n' "$fallback"
  fi
}

ensure_env_line() {
  key="$1"
  value="$2"
  if ! grep -q "^${key}=" .env 2>/dev/null; then
    printf '%s=%s\n' "$key" "$value" >> .env
  fi
}

need_cmd git
need_cmd docker

if ! docker compose version >/dev/null 2>&1; then
  echo "Docker Compose v2 is required. Please install Docker with the compose plugin first." >&2
  exit 1
fi

if [ -d "$INSTALL_DIR/.git" ]; then
  echo "Updating $INSTALL_DIR ..."
  git -C "$INSTALL_DIR" fetch origin "$BRANCH"
  git -C "$INSTALL_DIR" checkout "$BRANCH"
  git -C "$INSTALL_DIR" pull --ff-only origin "$BRANCH"
elif [ -e "$INSTALL_DIR" ]; then
  echo "$INSTALL_DIR already exists but is not a git repository." >&2
  exit 1
else
  echo "Cloning $REPO_URL ..."
  git clone --branch "$BRANCH" "$REPO_URL" "$INSTALL_DIR"
fi

cd "$INSTALL_DIR"
PROJECT_DIR="${PROJECT_DIR:-$(pwd -P)}"

if [ ! -f .env ]; then
  POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-$(random_hex 16)}"
  ADMIN_PASSWORD="${ADMIN_PASSWORD:-$(random_hex 8)}"
  AUTH_SECRET="${AUTH_SECRET:-$(random_hex 32)}"
  CORS_ALLOWED_ORIGINS="${CORS_ALLOWED_ORIGINS:-*}"
  APP_VERSION="${APP_VERSION:-$(git describe --tags --always 2>/dev/null || printf '%s' dev)}"
  SYSTEM_UPDATE_COMMAND="${SYSTEM_UPDATE_COMMAND:-$DEFAULT_UPDATE_COMMAND}"

  cat > .env <<EOF
HTTP_PORT=${HTTP_PORT}
PROJECT_DIR=${PROJECT_DIR}
TZ=Asia/Shanghai

POSTGRES_DB=${POSTGRES_DB:-sub2_expansion}
POSTGRES_USER=${POSTGRES_USER:-sub2_expansion}
POSTGRES_PASSWORD=${POSTGRES_PASSWORD}

ADMIN_USERNAME=${ADMIN_USERNAME}
ADMIN_PASSWORD=${ADMIN_PASSWORD}
AUTH_SECRET=${AUTH_SECRET}
AUTH_TOKEN_TTL_HOURS=${AUTH_TOKEN_TTL_HOURS:-24}
APP_VERSION=${APP_VERSION}
GITHUB_REPOSITORY=${GITHUB_REPOSITORY:-hepingan11/sub2-Expansion}
SYSTEM_UPDATE_COMMAND=${SYSTEM_UPDATE_COMMAND}

VITE_API_BASE=
CORS_ALLOWED_ORIGINS=${CORS_ALLOWED_ORIGINS}

CHECK_IN_DAILY_MAX_USERS=${CHECK_IN_DAILY_MAX_USERS:-20}

SUB2API_BASE_URL=${SUB2API_BASE_URL:-}
SUB2API_ADMIN_API_KEY=${SUB2API_ADMIN_API_KEY:-}
SUB2API_ADMIN_EMAIL=${SUB2API_ADMIN_EMAIL:-}
SUB2API_ADMIN_PASSWORD=${SUB2API_ADMIN_PASSWORD:-}
SUB2API_TIMEOUT_SECONDS=${SUB2API_TIMEOUT_SECONDS:-15}
SUB2API_TOKEN_REFRESH_ENABLED=${SUB2API_TOKEN_REFRESH_ENABLED:-true}
SUB2API_TOKEN_REFRESH_INTERVAL_SECONDS=${SUB2API_TOKEN_REFRESH_INTERVAL_SECONDS:-300}
EOF

  echo "Created .env with generated secrets."
else
  echo ".env already exists, keeping current configuration."
  ensure_env_line PROJECT_DIR "$PROJECT_DIR"
  ensure_env_line APP_VERSION "$(git describe --tags --always 2>/dev/null || printf '%s' dev)"
  ensure_env_line GITHUB_REPOSITORY "${GITHUB_REPOSITORY:-hepingan11/sub2-Expansion}"
  ensure_env_line SYSTEM_UPDATE_COMMAND "${SYSTEM_UPDATE_COMMAND:-$DEFAULT_UPDATE_COMMAND}"
fi

docker compose up -d --build

FINAL_HTTP_PORT="$(env_value HTTP_PORT "$HTTP_PORT")"
FINAL_POSTGRES_DB="$(env_value POSTGRES_DB "sub2_expansion")"
FINAL_POSTGRES_USER="$(env_value POSTGRES_USER "sub2_expansion")"
FINAL_POSTGRES_PASSWORD="$(env_value POSTGRES_PASSWORD "")"
FINAL_ADMIN_USERNAME="$(env_value ADMIN_USERNAME "$ADMIN_USERNAME")"
FINAL_ADMIN_PASSWORD="$(env_value ADMIN_PASSWORD "")"
FINAL_AUTH_SECRET="$(env_value AUTH_SECRET "")"

echo
echo "Install complete."
echo "Open: http://SERVER_IP:${FINAL_HTTP_PORT}/"
echo "Project directory: $(pwd)"
echo "Config file: $(pwd)/.env"
echo
echo "Generated/current credentials:"
echo "Admin username: ${FINAL_ADMIN_USERNAME}"
echo "Admin password: ${FINAL_ADMIN_PASSWORD}"
echo "PostgreSQL database: ${FINAL_POSTGRES_DB}"
echo "PostgreSQL username: ${FINAL_POSTGRES_USER}"
echo "PostgreSQL password: ${FINAL_POSTGRES_PASSWORD}"
echo "Auth secret: ${FINAL_AUTH_SECRET}"
echo
echo "Keep these values somewhere safe. They are also stored in .env."
