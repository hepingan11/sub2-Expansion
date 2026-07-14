#!/usr/bin/env sh
set -eu

REPO_URL="${REPO_URL:-https://github.com/hepingan11/sub2-Expansion.git}"
BRANCH="${BRANCH:-main}"
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
  od -An -N "$1" -tx1 /dev/urandom | tr -d ' \n'
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

can_prompt() {
  [ "${INSTALL_NONINTERACTIVE:-0}" != "1" ] && [ -r /dev/tty ] && [ -w /dev/tty ]
}

prompt_config() {
  label="$1"
  default_value="$2"
  value_type="$3"

  if ! can_prompt; then
    printf '%s\n' "$default_value"
    return
  fi

  if [ "$value_type" = "secret" ]; then
    default_description="随机生成"
  elif [ "$value_type" = "hidden-default" ] || [ "$value_type" = "optional-hidden-default" ]; then
    default_description="已启用"
  elif { [ "$value_type" = "optional" ] || [ "$value_type" = "optional-secret" ]; } && [ -z "$default_value" ]; then
    default_description="留空"
  else
    default_description="$default_value"
  fi

  printf '%s（默认：%s）使用默认值？[Y/n] ' "$label" "$default_description" > /dev/tty
  IFS= read -r use_default < /dev/tty || use_default=""
  case "$use_default" in
    n|N|no|NO)
      ;;
    *)
      printf '%s\n' "$default_value"
      return
      ;;
  esac

  while :; do
    if [ "$value_type" = "secret" ] || [ "$value_type" = "optional-secret" ]; then
      printf '请输入 %s：' "$label" > /dev/tty
      stty -echo < /dev/tty
      IFS= read -r custom_value < /dev/tty || custom_value=""
      stty echo < /dev/tty
      printf '\n' > /dev/tty
    else
      printf '请输入 %s：' "$label" > /dev/tty
      IFS= read -r custom_value < /dev/tty || custom_value=""
    fi
    if [ "$value_type" = "optional" ] || [ "$value_type" = "optional-secret" ] || [ "$value_type" = "optional-hidden-default" ] || [ -n "$custom_value" ]; then
      printf '%s\n' "$custom_value"
      return
    fi
    printf '该值不能为空，请重新输入。\n' > /dev/tty
  done
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
  CREATED_ENV=true
  echo "Configure Sub2 Expansion. Press Enter at each prompt to use the default or generated value."

  HTTP_PORT="$(prompt_config 'HTTP port' "${HTTP_PORT:-6779}" required)"
  TZ="$(prompt_config 'Timezone' "${TZ:-Asia/Shanghai}" required)"
  POSTGRES_DB="$(prompt_config 'PostgreSQL database name' "${POSTGRES_DB:-sub2_expansion}" required)"
  POSTGRES_USER="$(prompt_config 'PostgreSQL username' "${POSTGRES_USER:-sub2_expansion}" required)"
  POSTGRES_PASSWORD="$(prompt_config 'PostgreSQL password' "${POSTGRES_PASSWORD:-$(random_hex 16)}" secret)"
  ADMIN_USERNAME="$(prompt_config 'Admin username' "${ADMIN_USERNAME:-admin}" required)"
  ADMIN_PASSWORD="$(prompt_config 'Admin password' "${ADMIN_PASSWORD:-$(random_hex 8)}" secret)"
  AUTH_SECRET="$(prompt_config 'Auth signing secret' "${AUTH_SECRET:-$(random_hex 32)}" secret)"
  AUTH_TOKEN_TTL_HOURS="$(prompt_config 'Admin token lifetime in hours' "${AUTH_TOKEN_TTL_HOURS:-24}" required)"
  VITE_API_BASE="$(prompt_config 'Frontend API base URL (leave empty for the built-in proxy)' "${VITE_API_BASE:-}" optional)"
  CORS_ALLOWED_ORIGINS="$(prompt_config 'Allowed CORS origins' "${CORS_ALLOWED_ORIGINS:-*}" required)"
  FRONTEND_PUBLIC_URL="$(prompt_config 'Public frontend URL for social binding links' "${FRONTEND_PUBLIC_URL:-}" optional)"
  CHECK_IN_DAILY_MAX_USERS="$(prompt_config 'Daily check-in user limit' "${CHECK_IN_DAILY_MAX_USERS:-20}" required)"
  SUB2API_BASE_URL="$(prompt_config 'Sub2API base URL' "${SUB2API_BASE_URL:-}" optional)"
  SUB2API_ADMIN_API_KEY="$(prompt_config 'Sub2API Admin API Key' "${SUB2API_ADMIN_API_KEY:-}" optional-secret)"
  SUB2API_ADMIN_EMAIL="$(prompt_config 'Sub2API admin email' "${SUB2API_ADMIN_EMAIL:-}" optional)"
  SUB2API_ADMIN_PASSWORD="$(prompt_config 'Sub2API admin password' "${SUB2API_ADMIN_PASSWORD:-}" optional-secret)"
  SUB2API_TIMEOUT_SECONDS="$(prompt_config 'Sub2API timeout in seconds' "${SUB2API_TIMEOUT_SECONDS:-15}" required)"
  SUB2API_TOKEN_REFRESH_ENABLED="$(prompt_config 'Enable Sub2API token refresh' "${SUB2API_TOKEN_REFRESH_ENABLED:-true}" required)"
  SUB2API_TOKEN_REFRESH_INTERVAL_SECONDS="$(prompt_config 'Sub2API token refresh interval in seconds' "${SUB2API_TOKEN_REFRESH_INTERVAL_SECONDS:-300}" required)"
  GITHUB_REPOSITORY="$(prompt_config 'GitHub repository for update checks' "${GITHUB_REPOSITORY:-hepingan11/sub2-Expansion}" required)"
  SYSTEM_UPDATE_COMMAND="$(prompt_config 'System update command (leave empty to disable)' "${SYSTEM_UPDATE_COMMAND:-$DEFAULT_UPDATE_COMMAND}" optional-hidden-default)"
  APP_VERSION="${APP_VERSION:-$(git describe --tags --always 2>/dev/null || printf '%s' dev)}"

  cat > .env <<EOF
HTTP_PORT=${HTTP_PORT}
PROJECT_DIR=${PROJECT_DIR}
TZ=${TZ}

POSTGRES_DB=${POSTGRES_DB}
POSTGRES_USER=${POSTGRES_USER}
POSTGRES_PASSWORD=${POSTGRES_PASSWORD}

ADMIN_USERNAME=${ADMIN_USERNAME}
ADMIN_PASSWORD=${ADMIN_PASSWORD}
AUTH_SECRET=${AUTH_SECRET}
AUTH_TOKEN_TTL_HOURS=${AUTH_TOKEN_TTL_HOURS}
APP_VERSION=${APP_VERSION}
GITHUB_REPOSITORY=${GITHUB_REPOSITORY}
SYSTEM_UPDATE_COMMAND=${SYSTEM_UPDATE_COMMAND}

VITE_API_BASE=${VITE_API_BASE}
CORS_ALLOWED_ORIGINS=${CORS_ALLOWED_ORIGINS}
FRONTEND_PUBLIC_URL=${FRONTEND_PUBLIC_URL}

CHECK_IN_DAILY_MAX_USERS=${CHECK_IN_DAILY_MAX_USERS}

SUB2API_BASE_URL=${SUB2API_BASE_URL}
SUB2API_ADMIN_API_KEY=${SUB2API_ADMIN_API_KEY}
SUB2API_ADMIN_EMAIL=${SUB2API_ADMIN_EMAIL}
SUB2API_ADMIN_PASSWORD=${SUB2API_ADMIN_PASSWORD}
SUB2API_TIMEOUT_SECONDS=${SUB2API_TIMEOUT_SECONDS}
SUB2API_TOKEN_REFRESH_ENABLED=${SUB2API_TOKEN_REFRESH_ENABLED}
SUB2API_TOKEN_REFRESH_INTERVAL_SECONDS=${SUB2API_TOKEN_REFRESH_INTERVAL_SECONDS}
EOF

  echo "Created .env with generated secrets."
else
  CREATED_ENV=false
  echo ".env already exists, keeping current configuration."
  ensure_env_line PROJECT_DIR "$PROJECT_DIR"
  ensure_env_line APP_VERSION "$(git describe --tags --always 2>/dev/null || printf '%s' dev)"
  ensure_env_line GITHUB_REPOSITORY "${GITHUB_REPOSITORY:-hepingan11/sub2-Expansion}"
  ensure_env_line SYSTEM_UPDATE_COMMAND "${SYSTEM_UPDATE_COMMAND:-$DEFAULT_UPDATE_COMMAND}"
  ensure_env_line FRONTEND_PUBLIC_URL "${FRONTEND_PUBLIC_URL:-}"
fi

docker compose up -d --build

FINAL_HTTP_PORT="$(env_value HTTP_PORT "$HTTP_PORT")"

echo
echo "Install complete."
echo "Open: http://SERVER_IP:${FINAL_HTTP_PORT}/"
echo "Project directory: $(pwd)"
echo "Config file: $(pwd)/.env"

if [ "$CREATED_ENV" = true ]; then
  FINAL_ADMIN_USERNAME="$(env_value ADMIN_USERNAME "$ADMIN_USERNAME")"
  FINAL_ADMIN_PASSWORD="$(env_value ADMIN_PASSWORD "")"
  echo
  echo "Generated admin credentials (shown only for this first installation):"
  echo "Admin username: ${FINAL_ADMIN_USERNAME}"
  echo "Admin password: ${FINAL_ADMIN_PASSWORD}"
  echo "Store the password securely. PostgreSQL credentials and AUTH_SECRET are kept only in .env."
fi
