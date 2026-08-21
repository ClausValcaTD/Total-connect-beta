.PHONY: all build proto test clean

# Default target
all: proto build test

# Generate Go code from proto definitions
proto:
	@export PATH="/tmp/protoc/bin:$(shell go env GOPATH)/bin:$(PATH)"; \
	if command -v protoc >/dev/null 2>&1; then \
		protoc --go_out=. --go_opt=paths=source_relative \
			--go-grpc_out=. --go-grpc_opt=paths=source_relative \
			api/proto/totalconnect.proto; \
	else \
		echo "protoc not found in PATH"; exit 1; \
	fi

# Build core and cli modules
build:
	cd core && go build ./...
	cd cli && go build ./...

# Run tests across modules
test:
	cd core && go test ./...
	cd cli && go test ./...

clean:
	rm -f api/proto/*.pb.go
