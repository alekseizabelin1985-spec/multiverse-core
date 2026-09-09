# syntax=docker/dockerfile:1.7
# One image for every binary of the platform; compose picks the entry point
# with `command` (infrastructure.md §2.3).
#
# GO_VERSION comes from build/versions.env (F-6a, T-004); the default here is
# the toolchain pinned in go.mod so that a plain `docker build` works.
ARG GO_VERSION=1.26.8

FROM golang:${GO_VERSION}-bookworm AS builder
WORKDIR /src
ENV CGO_ENABLED=0 GOOS=linux GOFLAGS=-trimpath
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
ARG VERSION=dev
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -ldflags "-s -w -X main.version=${VERSION}" -o /out/ ./cmd/...

# Pinned by digest, not by the floating :nonroot tag (infrastructure.md §2.4);
# Dependabot (ecosystem docker, directory build/) bumps the line. Refresh with:
#   docker buildx imagetools inspect gcr.io/distroless/static-debian12:nonroot
# or, without docker: curl -sSI -H 'Accept: application/vnd.oci.image.index.v1+json' \
#   https://gcr.io/v2/distroless/static-debian12/manifests/nonroot | grep -i docker-content-digest
FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
# Everything under ./cmd is copied: wave 0 builds multiverse, mvctl follows in
# F-4c (T-010) and telegram-bot in EPIC-004, without touching this file.
COPY --from=builder /out/ /
# The data directories the binaries read (blueprints/, rules/, laws/, config/)
# are added by the tasks that create them: F-10 (rules, fixtures) and EPIC-003
# (blueprints, laws, absolute limits).
USER nonroot:nonroot
# No HEALTHCHECK here on purpose: one image serves every role and each role
# listens on its own port (gateway :8088, core :8090, memory :8082), so a URL
# baked into the image would mark two of the three unhealthy. The check lives
# per service in compose (infrastructure.md §2.3, T-004) and calls the binary
# itself — the image has neither shell nor curl:
#   healthcheck: test: ["CMD", "/multiverse", "health", "--url", "http://127.0.0.1:<port>/health"]
ENTRYPOINT ["/multiverse"]
