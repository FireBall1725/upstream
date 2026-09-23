# syntax=docker/dockerfile:1

# Web build. Static assets are the same on every architecture, so this runs once on the build platform.
FROM --platform=$BUILDPLATFORM node:24-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
ARG VERSION
RUN VERSION=${VERSION} npm run build

# Go build. Cross-compiles on the build platform rather than running the arm64 leg under QEMU.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
COPY --from=web /src/internal/ui/dist ./internal/ui/dist
# Empty by default so a local build reports 0.0.0-dev instead of a made-up version.
ARG VERSION
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -trimpath \
    -ldflags="-s -w ${VERSION:+-X 'github.com/fireball1725/upstream/internal/version.Version=${VERSION}'}" \
    -o /out/upstream ./cmd/upstream

# Runtime. Alpine rather than distroless because bump PRs shell out to git and helm.
FROM alpine:3.22
RUN apk add --no-cache ca-certificates git helm tzdata \
 && adduser -D -u 65532 -h /data upstream
COPY --from=build /out/upstream /usr/local/bin/upstream
ENV UPSTREAM_DATA_DIR=/data \
    HELM_CACHE_HOME=/data/helm/cache \
    HELM_CONFIG_HOME=/data/helm/config \
    HELM_DATA_HOME=/data/helm/data
USER 65532:65532
WORKDIR /data
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/upstream"]
