# build/Dockerfile

# ========== Стадия сборки ==========
FROM golang:1.25 AS builder
ARG SERVICE
ARG BUILD_CGO="0"
ARG GO_BUILD_TAGS=""
RUN echo "${SERVICE}"
# Установка git для go mod download (может потребоваться)
# Установка базовых зависимостей
RUN apt-get update && apt-get install -y --no-install-recommends \
    git ca-certificates build-essential curl \
    && rm -rf /var/lib/apt/lists/*

# ВСЕГДА создаём директорию (даже пустую при CGO=0)
RUN mkdir -p /opt/onnxruntime && \
    if [ "${BUILD_CGO}" = "1" ]; then \
      apt-get update && apt-get install -y --no-install-recommends \
        wget libgomp1 && \
      wget -O /tmp/onnxruntime.tgz \
        https://github.com/microsoft/onnxruntime/releases/download/v1.18.0/onnxruntime-linux-x64-1.18.0.tgz && \
      tar -xzf /tmp/onnxruntime.tgz -C /opt/onnxruntime --strip-components=1 && \
      rm -f /tmp/onnxruntime.tgz && \
      echo "ONNX Runtime installed to /opt/onnxruntime"; \
    else \
      echo "CGO disabled — skipping ONNX Runtime installation"; \
    fi

WORKDIR /app

# Копируем зависимости
COPY go.mod go.sum ./
RUN go mod download

# Копируем весь код
COPY . .

# Сборка с динамическим включением CGO
RUN echo "Building ${SERVICE} with CGO_ENABLED=${BUILD_CGO}, tags: '${GO_BUILD_TAGS}'" && \
    CGO_ENABLED=${BUILD_CGO} GOOS=linux go build -a -ldflags '-extldflags "-static"' \
      -tags "${GO_BUILD_TAGS}" \
      -o /bin/${SERVICE} \
      ./cmd/${SERVICE}

# ───────────────────────

# Можно вернуться к Alpine, так как CGO больше не нужен
FROM debian:bookworm-slim
ARG SERVICE
RUN echo "${SERVICE}"
#RUN apk --no-cache add ca-certificates tzdata curl
# Базовые зависимости
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates tzdata libgomp1 curl \
    && rm -rf /var/lib/apt/lists/*

# Если рантайм требует CGO — нужны системные библиотеки
ARG BUILD_CGO="0"
#RUN if [ "${BUILD_CGO}" = "1" ]; then \
#      apk add --no-cache musl-dev gcc g++ libc6-compat curl; \
#    fi

# Копируем ONNX Runtime ТОЛЬКО если он существует
COPY --from=builder --chown=root:root /opt/onnxruntime /opt/onnxruntime

# Регистрируем библиотеки в ldconfig, только если они есть
# Регистрируем библиотеки ТОЛЬКО если они есть (создаём директорию на всякий случай)
RUN mkdir -p /etc/ld.so.conf.d && \
    if [ -f "/opt/onnxruntime/lib/libonnxruntime.so.1.18.0" ]; then \
      echo "/opt/onnxruntime/lib" > /etc/ld.so.conf.d/onnxruntime.conf && \
      ldconfig && \
      echo "✓ ONNX Runtime libraries registered"; \
    else \
      echo "⊘ ONNX Runtime not needed — skipping ldconfig"; \
    fi


COPY --from=builder /bin/${SERVICE} ./${SERVICE}
RUN chmod +x ./${SERVICE}

CMD ["./${SERVICE}"]