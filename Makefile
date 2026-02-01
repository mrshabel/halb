.PHONY: run dev build

run:
	@go run cmd/main.go

dev:
	@echo "Starting test servers..."
	@go run examples/server/main.go -port 9000 -name auth-1 & echo $$! > /tmp/halb-auth1.pid
	@go run examples/server/main.go -port 9001 -name auth-2 & echo $$! > /tmp/halb-auth2.pid
	@go run examples/server/main.go -port 9002 -name billing-1 -delay 100ms & echo $$! > /tmp/halb-billing.pid
	@sleep 1
	@echo "Test servers running. Starting HALB..."
	@go run cmd/main.go -debug


build:
	@mkdir -p bin
	@CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/halb cmd/main.go
	@echo "Binary built: bin/halb"