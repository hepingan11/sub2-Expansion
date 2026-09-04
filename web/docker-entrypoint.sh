#!/bin/sh
set -e

# Executed automatically by the official nginx image entrypoint before nginx starts.

sanitize_id() {
    printf '%s' "$1" | tr -cd 'A-Za-z0-9-'
}

GA4_ID=$(sanitize_id "${ANALYTICS_GA4_ID:-}")
BAIDU_ID=$(sanitize_id "${ANALYTICS_BAIDU_ID:-}")
SUB2API_URL=$(printf '%s' "${SUB2API_URL:-}" | sed 's:/*$::')

if ! printf '%s' "$SUB2API_URL" | grep -Eq '^https?://[A-Za-z0-9.-]+(:[0-9]{1,5})?$'; then
    echo "SUB2API_URL must be an http(s) origin without a path" >&2
    exit 1
fi

cat > /usr/share/nginx/html/config.js <<EOF
window.__RUNTIME_CONFIG__ = {
  ANALYTICS_GA4_ID: "${GA4_ID}",
  ANALYTICS_BAIDU_ID: "${BAIDU_ID}",
  SUB2API_URL: "${SUB2API_URL}"
};
EOF

cat > /etc/nginx/conf.d/sub2api-proxy.inc <<EOF
location = /auth-api/api/v1/auth/login {
    limit_except POST { deny all; }
    proxy_pass ${SUB2API_URL}/api/v1/auth/login;
    proxy_set_header Host \$proxy_host;
    proxy_set_header X-Real-IP \$remote_addr;
    proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto \$scheme;
    proxy_ssl_server_name on;
}
location = /auth-api/api/v1/auth/logout {
    limit_except POST { deny all; }
    proxy_pass ${SUB2API_URL}/api/v1/auth/logout;
    proxy_set_header Host \$proxy_host;
    proxy_set_header X-Real-IP \$remote_addr;
    proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto \$scheme;
    proxy_ssl_server_name on;
}
location = /auth-api/api/v1/auth/me {
    limit_except GET { deny all; }
    proxy_pass ${SUB2API_URL}/api/v1/auth/me;
    proxy_set_header Host \$proxy_host;
    proxy_set_header X-Real-IP \$remote_addr;
    proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto \$scheme;
    proxy_ssl_server_name on;
}
location = /auth-api/api/v1/keys {
    limit_except GET { deny all; }
    proxy_pass ${SUB2API_URL}/api/v1/keys;
    proxy_set_header Host \$proxy_host;
    proxy_set_header X-Real-IP \$remote_addr;
    proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto \$scheme;
    proxy_ssl_server_name on;
}
location = /auth-api/api/v1/groups/available {
    limit_except GET { deny all; }
    proxy_pass ${SUB2API_URL}/api/v1/groups/available;
    proxy_set_header Host \$proxy_host;
    proxy_set_header X-Real-IP \$remote_addr;
    proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto \$scheme;
    proxy_ssl_server_name on;
}
location = /auth-api/v1/models {
    limit_except GET { deny all; }
    proxy_pass ${SUB2API_URL}/v1/models;
    proxy_set_header Host \$proxy_host;
    proxy_set_header X-Real-IP \$remote_addr;
    proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto \$scheme;
    proxy_ssl_server_name on;
}
EOF
