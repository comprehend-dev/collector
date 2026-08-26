collector: collector.go */*.go
	CGO_ENABLED=0 go build

# Point COLLECTOR_TEST_POSTGRES at a PostgreSQL connection string to run the collector test
# against a real database; without it that test is skipped.
test:
	go test ./...

release-internal:
	docker build -t us-central1-docker.pkg.dev/mystical-banner-176722/comprehend-dev/comprehend-agent:latest .
	docker push us-central1-docker.pkg.dev/mystical-banner-176722/comprehend-dev/comprehend-agent:latest

clean:
	rm collector

.PHONY: test clean
