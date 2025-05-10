# Detect OS and set executable extension
UNAME_S := $(shell uname -s)
ifeq ($(OS),Windows_NT)
	EXT := .exe
else
	ifeq ($(UNAME_S),Linux)
		EXT :=
	endif
	ifeq ($(UNAME_S),Darwin)
		EXT :=
	endif
endif

# Default variables
GO = go
PROTOC = protoc
SERVER_BINARY = server$(EXT)
CLIENT_BINARY = client$(EXT)
SERVER_SRC = ./cmd/pubsub_server/main.go
CLIENT_SRC = ./cmd/pubsub_client/main.go
LOG_DIR = logs
PROTO_DIR = proto
PROTO_FILES = $(wildcard $(PROTO_DIR)/*.proto)

.PHONY: all
all: build

.PHONY: build
build: proto
	@echo "Building server..."
	$(GO) build -o $(SERVER_BINARY) $(SERVER_SRC)
	@echo "Building client..."
	$(GO) build -o $(CLIENT_BINARY) $(CLIENT_SRC)

.PHONY: proto
proto:
	@echo "Generating Protobuf files..."
	@for proto in $(PROTO_FILES); do \
		$(PROTOC) --go_out=. --go-grpc_out=. $$proto; \
	done

.PHONY: run-server
run-server: build
	@echo "Running server..."
	@mkdir -p $(LOG_DIR)
	GRPC_ADDRESS=:50051 LOG_FILE=$(LOG_DIR)/server.log LOG_TO_FILE=true LOG_TO_CONSOLE=true ./$(SERVER_BINARY)

.PHONY: run-client
run-client: build
	@echo "Running client..."
	@mkdir -p $(LOG_DIR)
	GRPC_ADDRESS=localhost:50051 LOG_FILE=$(LOG_DIR)/client.log LOG_TO_FILE=true LOG_TO_CONSOLE=true ./$(CLIENT_BINARY)

.PHONY: test
test:
	@echo "Running tests..."
	$(GO) test ./... -v

.PHONY: fmt
fmt:
	@echo "Formatting code..."
	$(GO) fmt ./...

.PHONY: deps
deps:
	@echo "Installing dependencies..."
	$(GO) install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	$(GO) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

.PHONY: docker-build
docker-build:
	@echo "Building Docker images..."
	docker-compose build

.PHONY: docker-up
docker-up: docker-build
	@echo "Starting services with Docker Compose..."
	@mkdir -p $(LOG_DIR)
	docker-compose up

.PHONY: docker-down
docker-down:
	@echo "Stopping Docker Compose services..."
	docker-compose down

.PHONY: clean
clean:
	@echo "Cleaning up..."
	rm -f server client server.exe client.exe
	rm -rf $(LOG_DIR)

.PHONY: help
help:
	@echo "Available commands:"
	@echo "  make build        - Build server and client binaries"
	@echo "  make proto        - Generate Protobuf files"
	@echo "  make run-server   - Run server locally"
	@echo "  make run-client   - Run client locally"
	@echo "  make test         - Run unit tests"
	@echo "  make fmt          - Format code"
	@echo "  make lint         - Run linter"
	@echo "  make deps         - Install dependencies"
	@echo "  make docker-build - Build Docker images"
	@echo "  make docker-up    - Start Docker Compose services"
	@echo "  make docker-down  - Stop Docker Compose services"
	@echo "  make clean        - Remove binaries and logs"
	@echo "  make help         - Show this help message"
