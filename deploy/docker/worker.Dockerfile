# syntax=docker/dockerfile:1
#
# Multi-stage build for the worker. Stage 1 compiles a static binary; stage 2 is
# a tiny runtime image that adds only ffmpeg. Build from the repo root:
#   docker build -f deploy/docker/worker.Dockerfile -t fiapx-worker .

# --- build stage ----------------------------------------------------------
FROM golang:1.25-alpine AS build
WORKDIR /src

# Download dependencies first so this layer is cached until go.mod/go.sum change.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .
# CGO disabled -> a fully static binary that runs on a bare Alpine image.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -o /out/worker ./cmd/worker

# --- runtime stage --------------------------------------------------------
FROM alpine:3.20
RUN apk add --no-cache ffmpeg ca-certificates
COPY --from=build /out/worker /usr/local/bin/worker

# Scratch space for downloads/frames/zips.
ENV WORK_DIR=/tmp
ENTRYPOINT ["worker"]
