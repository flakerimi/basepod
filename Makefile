# BasePod Makefile

VERSION ?= 0.0.0-dev
GOFLAGS := -ldflags "-X main.version=$(VERSION)"
BIN := bin
DIST := dist
GOOS ?= darwin
GOARCH ?= arm64

.PHONY: all build server cli web test lint sqlc goose-up run clean release tidy docs

all: build

build: server cli ## build both binaries

server: ## build basepod-server
	@mkdir -p $(BIN)
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build $(GOFLAGS) -o $(BIN)/basepod-server ./cmd/basepod-server

cli: ## build basepod CLI
	@mkdir -p $(BIN)
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build $(GOFLAGS) -o $(BIN)/basepod ./cmd/basepod

web: ## build the Vue SPA
	cd web && pnpm install && pnpm build

test: ## run tests
	./scripts/test-installers.sh
	go test ./... -race -count=1

lint: ## run linters
	go vet ./...

sqlc: ## regenerate sqlc code
	sqlc generate

goose-up: ## apply migrations (path arg expected)
	goose -dir internal/store/migrations sqlite3 $(DB) up

tidy:
	go mod tidy

run: ## run the server locally
	go run ./cmd/basepod-server

clean:
	rm -rf $(BIN) $(DIST)

release: clean web ## build release artifacts
	./scripts/build.sh $(VERSION)

docs: cli ## regenerate the CLI markdown reference
	@mkdir -p docs/cli
	@./$(BIN)/basepod docs docs/cli
