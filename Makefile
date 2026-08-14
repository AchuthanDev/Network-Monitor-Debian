.PHONY: test lint build docker-up docker-down frontend-test backend-test collector-test

test: backend-test collector-test frontend-test

backend-test:
	cd apps/backend && go test ./...

collector-test:
	cd features/network-usage && go test ./...
	cd apps/collector && go test ./...

frontend-test:
	cd apps/frontend && npm test

lint:
	cd apps/backend && go test ./...
	cd features/network-usage && go test ./...
	cd apps/collector && go test ./...
	cd apps/frontend && npm run lint

build:
	cd apps/backend && go build ./cmd/api
	cd apps/collector && go build ./cmd/collector
	cd apps/frontend && npm run build

docker-up:
	docker compose -f deployments/docker/compose.yml up -d --build

docker-down:
	docker compose -f deployments/docker/compose.yml down
