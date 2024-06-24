FROM golang:1.22
COPY ./ ./
RUN make

FROM scratch
COPY --from=0 /go/agent /
ENTRYPOINT ["/agent"]
