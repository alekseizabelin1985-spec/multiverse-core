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
# Both version variables are stamped (decision ОВ-30): main.version is what
# `mvctl version` prints, logging.Version is the version field of every log
# line (§7.1, NFR-041). Stamping only one leaves the other at "dev".
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -ldflags "-s -w -X main.version=${VERSION} -X multiverse-core.io/shared/logging.Version=${VERSION}" -o /out/ ./cmd/...
# The empty data directory of the gateway, copied into the runtime image below:
# the distroless image has no shell to create it there.
RUN mkdir -m 0700 /gateway-data

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
# are added by the tasks that read them; EPIC-003 adds blueprints, laws and the
# absolute limits.
# rules/: the context state does not start without the laws of the rule book
# MV_RULES_PATH names, rules/dark-forest.yaml relative to the working directory
# by default (T-471). The working directory is set here rather than inherited
# from the base image, so that the default resolves to the copy below whatever
# the image under it says.
WORKDIR /home/nonroot
COPY rules/ ./rules/
# MV_GATEWAY_DATA_DIR of compose (T-303): links.db holds player identifiers,
# and the store refuses a directory open to the group or others (ADR-019 p. 1).
# Owned by nonroot (65532) with 0700, so the process can create its files, and
# because an empty named volume mounted at /data (gateway-data) takes the owner
# and mode of this directory on its first mount; without it the volume is
# root 0755 and the gateway does not start. A volume created before this line
# keeps its root 0755: recreate it (Docs/ops/runbook.md, section 2).
COPY --from=builder --chown=65532:65532 --chmod=0700 /gateway-data /data
USER nonroot:nonroot
# No HEALTHCHECK here on purpose: one image serves every role and each role
# listens on its own port (gateway :8088, core :8090, memory :8082), so a URL
# baked into the image would mark two of the three unhealthy. The check lives
# per service in compose (infrastructure.md §2.3, T-004) and calls the binary
# itself — the image has neither shell nor curl:
#   healthcheck: test: ["CMD", "/multiverse", "health", "--url", "http://127.0.0.1:<port>/health"]
ENTRYPOINT ["/multiverse"]
