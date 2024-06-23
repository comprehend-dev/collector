agent: agent.go
	CGO_ENABLED=0 go build

test:
	../tools/with-api-service-and-std.raku run go test

clean:
	rm agent

.PHONY: test clean
