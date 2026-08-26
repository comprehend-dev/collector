collector: collector.go */*.go
	CGO_ENABLED=0 go build

# Point COLLECTOR_TEST_POSTGRES at a PostgreSQL connection string to run the collector test
# against a real database; without it that test is skipped.
test:
	go test ./...

# Refresh after changing dependencies; needs go install github.com/google/go-licenses@latest
third-party-licenses:
	go-licenses report ./... --ignore github.com/comprehend-dev/collector \
		--template third-party-licenses.tpl > THIRD-PARTY-LICENSES

clean:
	rm collector

.PHONY: test third-party-licenses clean
