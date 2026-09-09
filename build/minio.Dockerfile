# syntax=docker/dockerfile:1.7
# MinIO built from source (ADR-021 variant B, infrastructure.md §2.5).
#
# Why: MinIO Inc. stopped publishing community images in Oct 2025 (last one is
# RELEASE.2025-09-07T16-13-09Z) and archived the repository in Feb 2026, while
# the fix for CVE-2025-62506 shipped as source only, in the tag pinned below.
#
# Build it through `make minio-image`, which passes every ARG from
# build/versions.env. The defaults here keep a bare `docker build` working and
# must stay equal to that file.
ARG MINIO_BUILDER_IMAGE=golang:1.26.8-bookworm

FROM ${MINIO_BUILDER_IMAGE} AS builder
ARG MINIO_TAG=RELEASE.2025-10-15T17-29-55Z
# The owner's fork is the first line of defence against the sources going away
# (infrastructure.md §2.5); until it exists, MINIO_REPO points at the archived
# upstream, which still serves the tag read-only. The second line is the source
# tarball in backups/minio-src-<tag>.tar.gz (outside Git).
ARG MINIO_REPO=https://github.com/minio/minio.git
# The tag is mutable; the commit it pointed at on 2026-09-09 is pinned and
# verified fail-closed (SEC-25; review T-004, Mi-5). Update both together.
ARG MINIO_COMMIT=9e49d5e7a648
WORKDIR /src
RUN git clone --depth 1 --branch "${MINIO_TAG}" "${MINIO_REPO}" .  && test "$(git rev-parse HEAD | cut -c1-12)" = "${MINIO_COMMIT}" 
ENV CGO_ENABLED=0 GOOS=linux GOFLAGS=-trimpath
# gen-ldflags.go stamps what `minio --version` prints. Without MINIO_RELEASE it
# prefixes the build "DEVELOPMENT." (its default), and without an explicit
# version argument it derives one from the commit time — so both are passed:
# the tag minus its RELEASE. prefix is exactly the version the script expects,
# which makes the stamp independent of the commit metadata of the clone.
ENV MINIO_RELEASE=RELEASE
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -tags kqueue \
      -ldflags "$(go run buildscripts/gen-ldflags.go "${MINIO_TAG#RELEASE.}")" \
      -o /out/minio .

# alpine, not distroless: the healthcheck needs wget and the server must not run
# as root. Resulting image is ~120 MB.
FROM alpine:3.22
RUN adduser -D -u 1000 minio && mkdir -p /data && chown minio:minio /data
COPY --from=builder /out/minio /usr/bin/minio
USER minio
VOLUME /data
EXPOSE 9000 9001
HEALTHCHECK --interval=10s --timeout=5s --start-period=20s \
  CMD wget -qO- http://127.0.0.1:9000/minio/health/live >/dev/null || exit 1
ENTRYPOINT ["/usr/bin/minio"]
CMD ["server", "/data", "--console-address", ":9001"]
