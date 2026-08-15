GO_IMAGE ?= golang:1.23.10-alpine
GO_DOCKER = docker run --rm -v $(CURDIR):/src -w /src -e GOCACHE=/tmp/go-cache $(GO_IMAGE)

.PHONY: test lint build docker-up docker-down frontend-test backend-test collector-test gateway-test gateway-sim-test go-test go-build frontend-build

test: go-test gateway-sim-test frontend-test

go-test:
	$(GO_DOCKER) sh scripts/test-go.sh

backend-test: go-test

collector-test: go-test

gateway-test: go-test

gateway-sim-test:
	docker build -f deployments/docker/gateway-sim.Dockerfile -t network-monitor-gateway-sim:local .
	docker run --rm --privileged --network none network-monitor-gateway-sim:local

frontend-test:
	cd apps/frontend && npm test

lint: go-test
	cd apps/frontend && npm run lint

build: go-build frontend-build

go-build:
	$(GO_DOCKER) sh -eu -c 'cd apps/backend && go build -o /tmp/network-monitor-api ./cmd/api && cd ../collector && go build -o /tmp/network-monitor-collector ./cmd/collector'

frontend-build:
	cd apps/frontend && npm run build

docker-up:
	docker compose -f deployments/docker/compose.yml up -d --build

docker-down:
	docker compose -f deployments/docker/compose.yml down
