# =============================================================================
# build/legacy.Dockerfile — the two as-is services of the `legacy` profile
# (infrastructure.md v0.3 §1.3, D-3, ADR-004 add. 8; task T-008)
# =============================================================================
# Usage (never a bare `docker build .` from the repository root):
#   make legacy-src        materialises build/.legacy-src/ from LEGACY_SRC_REF
#   docker build -f build/legacy.Dockerfile --build-arg SERVICE=<name> \
#                build/.legacy-src
# `docker compose --profile legacy build` does the same through the `context:
# build/.legacy-src` of the two services.
#
# WHY A SEPARATE CONTEXT. This file is the as-is root Dockerfile
# (services/_archive/build/Dockerfile.root), and it builds the frozen services
# the way they were built before EPIC-001: a Go workspace over the root module
# plus one service directory. That layout no longer exists in the work tree:
#   * F-2 (T-003) deleted go.work and turned the repository into one Go 1.26
#     module — this file's `COPY go.work` had nothing left to copy;
#   * F-4a (T-005) replaced shared/eventbus with the C-01 bus, so the frozen
#     code's `eventbus.EventBus` is gone;
#   * F-3 (T-002) moved shared/{config,minio,oracle,spatial} into
#     services/_archive/shared/ (a different module path) and dropped the
#     effectful shared/agent/tools (SEC-18, ADR-001 add. 5).
# Verified in T-008 by reconstructing the layout by hand: the frozen code does
# not compile against the current shared/ (undefined eventbus.EventBus,
# tools.WorldTool, tools.EntityTool, tools.NarrativeTool). Editing it is not an
# option — the services are frozen — so the build context is the last commit at
# which they did compile, exported by `make legacy-src`. Nothing is deleted and
# nothing is un-frozen: U-1 holds.
#
# The exported context carries only go.work, go.mod, go.sum, shared/ and the
# two service directories (see the Makefile) — the as-is .mcp.env of that
# commit is deliberately not among them.
# =============================================================================

# The frozen services declare `go 1.24.x`; they are not modernised, they are
# kept working until S5 removes the profile (EPIC-003 I2). Pin from
# build/versions.env.
ARG LEGACY_GO_IMAGE=golang:1.24.11-bookworm
# The runtime base is pinned by digest for the same reason as every other pin
# in build/versions.env: `debian:bookworm-slim` moves with the branch
# (NFR-071, review T-008 Mi-6).
ARG LEGACY_RUNTIME_IMAGE=debian:bookworm-slim@sha256:88200866dfff7ea7f5cbcb6ec7c8a701889efe6fe859fe64d6990e4b07ea4171
# The one download of this build that does not come from a registry.
ARG LEGACY_ONNXRUNTIME_VERSION=1.18.0
ARG LEGACY_ONNXRUNTIME_SHA256=fa4d11b3fa1b2bf1c3b2efa8f958634bc34edc95e351ac2a0408c6ad5c5504f0

FROM ${LEGACY_GO_IMAGE} AS builder
ARG SERVICE
ARG BUILD_CGO="0"
ARG GO_BUILD_TAGS=""
ARG LEGACY_ONNXRUNTIME_VERSION
ARG LEGACY_ONNXRUNTIME_SHA256

RUN apt-get update && apt-get install -y --no-install-recommends \
    git ca-certificates build-essential curl \
    && rm -rf /var/lib/apt/lists/*

# ONNX Runtime is needed only by semantic-memory (chroma_v2_enabled + CGO).
RUN mkdir -p /opt/onnxruntime && \
    if [ "${BUILD_CGO}" = "1" ]; then \
      apt-get update && apt-get install -y --no-install-recommends wget libgomp1 && \
      wget -O /tmp/onnxruntime.tgz \
        "https://github.com/microsoft/onnxruntime/releases/download/v${LEGACY_ONNXRUNTIME_VERSION}/onnxruntime-linux-x64-${LEGACY_ONNXRUNTIME_VERSION}.tgz" && \
      echo "${LEGACY_ONNXRUNTIME_SHA256}  /tmp/onnxruntime.tgz" | sha256sum -c - && \
      tar -xzf /tmp/onnxruntime.tgz -C /opt/onnxruntime --strip-components=1 && \
      rm -f /tmp/onnxruntime.tgz && \
      echo "ONNX Runtime installed"; \
    else \
      echo "CGO disabled — skipping ONNX Runtime"; \
    fi

WORKDIR /workspace

# The as-is workspace, as exported by `make legacy-src`.
COPY go.mod go.sum ./
COPY shared/ ./shared/
COPY services/${SERVICE}/ ./services/${SERVICE}/

# A workspace over the root module, every shared/* module and this one service.
# The exported go.work names all 15 as-is services and most of them are not in
# this context, so it is regenerated rather than reused. The shared packages
# must stay in it: in the as-is layout shared/{agent, agent/tools, config,
# eventbus, minio, oracle, spatial} are each their own module, and the as-is
# root Dockerfile's `use (. ./services/X)` left the build with "no required
# module provides package multiverse-core.io/shared/oracle" (verified in T-008).
RUN { \
      echo 'go 1.24.0'; \
      echo; \
      echo 'use ('; \
      echo '	.'; \
      find shared -name go.mod -printf '	./%h\n' | sort; \
      printf '\t./services/%s\n' "${SERVICE}"; \
      echo ')'; \
    } > go.work && cat go.work

RUN cd services/${SERVICE} && go mod download

# Static linking only without CGO; the chroma_v2 + ONNX build needs dynamic.
RUN echo "Building ${SERVICE} CGO_ENABLED=${BUILD_CGO} tags='${GO_BUILD_TAGS}'" && \
    cd services/${SERVICE} && \
    if [ "${BUILD_CGO}" = "1" ]; then \
      CGO_ENABLED=1 GOOS=linux \
        CGO_CFLAGS="-I/opt/onnxruntime/include" \
        CGO_LDFLAGS="-L/opt/onnxruntime/lib -lonnxruntime" \
        go build -a -tags "${GO_BUILD_TAGS}" -o /bin/${SERVICE} ./cmd/ ; \
    else \
      CGO_ENABLED=0 GOOS=linux go build \
        -a -ldflags '-extldflags "-static"' \
        -tags "${GO_BUILD_TAGS}" -o /bin/${SERVICE} ./cmd/ ; \
    fi

# ========== Runtime stage ==========
FROM ${LEGACY_RUNTIME_IMAGE}
ARG SERVICE
ARG BUILD_CGO="0"
ARG LEGACY_ONNXRUNTIME_VERSION

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates tzdata libgomp1 curl \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder --chown=root:root /opt/onnxruntime /opt/onnxruntime

RUN mkdir -p /etc/ld.so.conf.d && \
    if [ -f "/opt/onnxruntime/lib/libonnxruntime.so.${LEGACY_ONNXRUNTIME_VERSION}" ]; then \
      echo "/opt/onnxruntime/lib" > /etc/ld.so.conf.d/onnxruntime.conf && \
      ldconfig && \
      echo "ONNX Runtime libraries registered"; \
    else \
      echo "ONNX Runtime not needed — skipping ldconfig"; \
    fi

COPY --from=builder /bin/${SERVICE} /legacy-service
RUN chmod +x /legacy-service

ENTRYPOINT ["/legacy-service"]
