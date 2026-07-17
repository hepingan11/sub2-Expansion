# syntax=docker/dockerfile:1

FROM golang:1.25-alpine AS build

WORKDIR /src

ARG APP_VERSION=v0.2

RUN apk add --no-cache ca-certificates docker-cli docker-cli-compose git tzdata

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/redeem-code-system-go .

FROM alpine:3.22

ARG APP_VERSION=v0.2

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=build /out/redeem-code-system-go /app/redeem-code-system-go

ENV GIN_MODE=release \
    SERVER_PORT=8625 \
    APP_VERSION=${APP_VERSION} \
    TZ=Asia/Shanghai

EXPOSE 8625

HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8625/healthz >/dev/null || exit 1

ENTRYPOINT ["/app/redeem-code-system-go"]
