# syntax=docker/dockerfile:1
#
# Generic multi-stage build for the pure-Go services (gateway, notifier) — those
# that don't need ffmpeg. Pass the service name as a build arg:
#   docker build -f deploy/docker/service.Dockerfile --build-arg SERVICE=gateway -t fiapx-gateway .

ARG SERVICE

# --- build stage ----------------------------------------------------------
FROM golang:1.25-alpine AS build
ARG SERVICE
WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -o /out/app ./cmd/${SERVICE}

# --- runtime stage --------------------------------------------------------
FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /out/app /usr/local/bin/app
ENTRYPOINT ["app"]
