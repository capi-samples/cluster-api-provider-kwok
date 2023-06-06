# syntax=docker/dockerfile:1.4


# Build the manager binary
ARG builder_image

# Build architecture
ARG ARCH

# Ignore Hadolint rule "Always tag the version of an image explicitly."
# It's an invalid finding since the image is explicitly set in the Makefile.
# https://github.com/hadolint/hadolint/wiki/DL3006
# hadolint ignore=DL3006
FROM ${builder_image} as builder
WORKDIR /workspace

# Run this with docker build --build-arg goproxy=$(go env GOPROXY) to override the goproxy
ARG goproxy=https://proxy.golang.org
# Run this with docker build --build-arg package=./controlplane or --build-arg package=./bootstrap
ENV GOPROXY=$goproxy

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
#RUN mkdir -p third_party/kwok/stages
COPY third_party/kwok/go.mod third_party/kwok/go.mod
COPY third_party/kwok/go.sum third_party/kwok/go.sum
#COPY third_party/kwok/stages/*.yaml third_party/kwok/stages
#RUN ls ./third_party/kwok/stages

# Cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy the sources
COPY ./ ./

# Build
ARG package=.
ARG ARCH
ARG ldflags

# Do not force rebuild of up-to-date packages (do not use -a) and use the compiler cache folder
RUN --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux GOARCH=${ARCH} \
    go build -trimpath -ldflags "${ldflags} -extldflags '-static'" \
    -o manager ${package}

COPY scripts/get-docker-cli.sh get-docker-cli.sh
RUN ./get-docker-cli.sh ${ARCH} out


# Production image
#FROM gcr.io/distroless/static:nonroot-${ARCH}
FROM alpine:3.17

RUN apk add --no-cache \
		ca-certificates \
# DOCKER_HOST=ssh://... -- https://github.com/docker/cli/pull/1014
		openssh-client

LABEL org.opencontainers.image.source=https://github.com/capi-samples/cluster-api-provider-kwok
WORKDIR /
COPY --from=builder /workspace/manager .
COPY --from=builder /workspace/out/docker /usr/local/bin/docker

RUN mkdir -p /usr/local/libexec/docker/cli-plugins
COPY --from=builder /workspace/out/docker-compose /usr/local/libexec/docker/cli-plugins/docker-compose
RUN docker --version && /usr/local/libexec/docker/cli-plugins/docker-compose --version

# Use uid of nonroot user (65532) because kubernetes expects numeric user when applying pod security policies
USER 65532
ENTRYPOINT ["/manager"]
