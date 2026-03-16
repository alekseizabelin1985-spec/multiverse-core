# Dockerfile for evolution-watcher service (CGO + ONNX Runtime)
FROM golang:1.24 AS builder
ARG SERVICE=evolution-watcher
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

COPY go.work go.work.sum* ./
COPY go.mod go.sum ./
COPY shared/ ./shared/
COPY services/evolution-watcher/ ./services/evolution-watcher/

RUN cd services/evolution-watcher && go mod download

RUN echo "Building evolution-watcher CGO_ENABLED=${BUILD_CGO}" && \
    cd services/evolution-watcher && \
    CGO_ENABLED=${BUILD_CGO} GOOS=linux go build \
      -a -ldflags '-extldflags "-static"' \
      -tags "${GO_BUILD_TAGS}" \
      -o /bin/evolution-watcher \
      ./cmd/

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates tzdata libgomp1 curl \
    && rm -rf /var/lib/apt/lists/*

ARG BUILD_CGO="0"
COPY --from=builder --chown=root:root /opt/onnxruntime /opt/onnxruntime

RUN mkdir -p /etc/ld.so.conf.d && \
    if [ -f "/opt/onnxruntime/lib/libonnxruntime.so.1.18.0" ]; then \
      echo "/opt/onnxruntime/lib" > /etc/ld.so.conf.d/onnxruntime.conf && \
      ldconfig && echo "ONNX Runtime registered"; \
    else \
      echo "ONNX Runtime not needed"; \
    fi

COPY --from=builder /bin/evolution-watcher ./evolution-watcher
RUN chmod +x ./evolution-watcher
CMD ["./evolution-watcher"]
