FROM scratch
COPY agent /
ENTRYPOINT ["/agent"]
