.PHONY: web build test

# Build the dashboard into internal/webui/dist/app, where the Go build embeds it.
web:
	cd web && npm ci && npm run build

# Build the server with the dashboard embedded.
build: web
	go build -o bin/server ./cmd/server

test:
	go test ./...
	cd web && npm test
