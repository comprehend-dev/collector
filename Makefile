agent: agent.go */*.go
	CGO_ENABLED=0 go build

test:
	ENV_MODE=test COMPREHEND_SERVICES_TO_RUN=api,ingestion ../tools/services-with-db.raku run go test

release-internal:
	docker build -t us-central1-docker.pkg.dev/mystical-banner-176722/comprehend-dev/comprehend-agent:latest .
	docker push us-central1-docker.pkg.dev/mystical-banner-176722/comprehend-dev/comprehend-agent:latest

clean:
	rm agent

.PHONY: test clean
