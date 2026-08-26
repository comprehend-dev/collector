FROM golang:1.23
ARG VERSION=dev
COPY ./ ./
RUN make VERSION=$VERSION

FROM alpine:3.16
COPY --from=0 /go/collector /
ENTRYPOINT ["/collector"]
