# syntax=docker/dockerfile:1

FROM golang:1.25-alpine AS build

WORKDIR /src

RUN apk add --no-cache ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/redeem-code-system-go .

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=build /out/redeem-code-system-go /app/redeem-code-system-go

ENV GIN_MODE=release \
    SERVER_PORT=8625 \
    TZ=Asia/Shanghai

EXPOSE 8625

HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8625/healthz >/dev/null || exit 1

ENTRYPOINT ["/app/redeem-code-system-go"]
