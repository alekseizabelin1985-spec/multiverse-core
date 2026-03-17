# Dockerfile — workspace-aware build for any service
# Usage: docker build --build-arg SERVICE=<name> .

# ========== Builder stage ==========
FROM golang:1.24 AS builder
ARG SERVICE
ARG BUILD_CGO="0"
ARG GO_BUILD_TAGS=""

RUN apt-get update && apt-get install -y --no-install-recommends \
    git ca-certificates build-essential curl \
    && rm -rf /var/lib/apt/lists/*

RUN mkdir -p /opt/onnxruntime && \
    if [ "${BUILD_CGO}" = "1" ]; then \
      apt-get update && apt-get install -y --no-install-recommends wget libgomp1 && \
      wget -O /tmp/onnxruntime.tgz \
        https://github.com/microsoft/onnxruntime/releases/download/v1.18.0/onnxruntime-linux-x64-1.18.0.tgz && \
      tar -xzf /tmp/onnxruntime.tgz -C /opt/onnxruntime --strip-components=1 && \
      rm -f /tmp/onnxruntime.tgz && \
      echo "ONNX Runtime installed"; \
    else \
      echo "CGO disabled — skipping ONNX Runtime"; \
    fi

WORKDIR /workspace

# Copy workspace file and root module (shared packages)
COPY go.work go.work.sum* ./
COPY go.mod go.sum ./
COPY shared/ ./shared/

# Copy only the target service module
COPY services/${SERVICE}/ ./services/${SERVICE}/

# Replace go.work with a minimal version: root module + target service only
RUN printf 'go 1.24.0\n\nuse (\n\t.\n\t./services/%s\n)\n' "${SERVICE}" > go.work && rm -f go.work.sum

# Download dependencies for this service
RUN cd services/${SERVICE} && go mod download

# Build the service
RUN echo "Building ${SERVICE} CGO_ENABLED=${BUILD_CGO} tags='${GO_BUILD_TAGS}'" && \
    cd services/${SERVICE} && \
    CGO_ENABLED=${BUILD_CGO} GOOS=linux go build \
      -a -ldflags '-extldflags "-static"' \
      -tags "${GO_BUILD_TAGS}" \
      -o /bin/${SERVICE} \
      ./cmd/

# ========== Runtime stage ==========
FROM debian:bookworm-slim
ARG SERVICE

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates tzdata libgomp1 curl \
    && rm -rf /var/lib/apt/lists/*

ARG BUILD_CGO="0"

COPY --from=builder --chown=root:root /opt/onnxruntime /opt/onnxruntime

RUN mkdir -p /etc/ld.so.conf.d && \
    if [ -f "/opt/onnxruntime/lib/libonnxruntime.so.1.18.0" ]; then \
      echo "/opt/onnxruntime/lib" > /etc/ld.so.conf.d/onnxruntime.conf && \
      ldconfig && \
      echo "ONNX Runtime libraries registered"; \
    else \
      echo "ONNX Runtime not needed — skipping ldconfig"; \
    fi

COPY --from=builder /bin/${SERVICE} ./${SERVICE}
RUN chmod +x ./${SERVICE}

CMD ["./${SERVICE}"]
