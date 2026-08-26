FROM golang:1.23
COPY ./ ./
RUN make

FROM alpine:3.16
COPY --from=0 /go/collector /
ENTRYPOINT ["/collector"]
