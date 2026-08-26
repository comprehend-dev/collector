collector: collector.go */*.go
	CGO_ENABLED=0 go build

# Point COLLECTOR_TEST_POSTGRES at a PostgreSQL connection string to run the collector test
# against a real database; without it that test is skipped.
test:
	go test ./...

clean:
	rm collector

.PHONY: test clean
