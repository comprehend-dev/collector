FROM golang:1.22
COPY ./ ./
RUN make

FROM alpine:3.16
COPY --from=0 /go/agent /
ENTRYPOINT ["/agent"]
