# Build on the runner's own architecture and cross-compile, so that building for arm64 needs no
# emulation; the collector is pure Go with cgo disabled.
FROM --platform=$BUILDPLATFORM golang:1.23 AS build
ARG VERSION=dev
ARG TARGETOS
ARG TARGETARCH
COPY ./ ./
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH make VERSION=$VERSION

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /go/collector /collector
COPY --from=build /go/LICENSE /go/THIRD-PARTY-LICENSES /
ENTRYPOINT ["/collector"]
