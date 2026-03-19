.PHONY: build build-all test lint fmt clean install release

BINARY_NAME=gitdual
GITHUB_REPO=florianjs/gitdual
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-s -w -X main.Version=$(VERSION)"

build:
	go build $(LDFLAGS) -o $(BINARY_NAME) ./cmd/gitdual

build-all:
	mkdir -p bin/
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-amd64 ./cmd/gitdual
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-arm64 ./cmd/gitdual
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-amd64 ./cmd/gitdual
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-arm64 ./cmd/gitdual
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-windows-amd64.exe ./cmd/gitdual

test:
	go test -v ./...

test-coverage:
	go test -cover ./...

lint:
	golangci-lint run

lint-fix:
	golangci-lint run --fix

fmt:
	go fmt ./...

vet:
	go vet ./...

tidy:
	go mod tidy

clean:
	rm -rf bin/
	rm -f $(BINARY_NAME)

run:
	go run ./cmd/gitdual

install: build
	@echo "Installing gitdual to /usr/local/bin..."
	@if [ -w /usr/local/bin ]; then \
		mv $(BINARY_NAME) /usr/local/bin/$(BINARY_NAME); \
	else \
		sudo mv $(BINARY_NAME) /usr/local/bin/$(BINARY_NAME); \
	fi
	@echo "✓ Installed. Run 'gitdual --help' to get started."

## Release: push public, build binaries, create GH release, tag public remote.
## Usage: make release VERSION=v1.0.0
release: build-all
	@if [ -z "$(VERSION)" ]; then echo "Usage: make release VERSION=v1.0.0"; exit 1; fi
	@echo "Pushing to public remote..."
	go run ./cmd/gitdual push public --force "Release $(VERSION)"
	@echo "Creating GitHub release $(VERSION)..."
	gh release create $(VERSION) \
		--repo $(GITHUB_REPO) \
		--title "$(VERSION)" \
		--generate-notes \
		bin/gitdual-darwin-amd64 \
		bin/gitdual-darwin-arm64 \
		bin/gitdual-linux-amd64 \
		bin/gitdual-linux-arm64 \
		bin/gitdual-windows-amd64.exe

install-local: build
	@echo "Installing gitdual to $(GOPATH)/bin..."
	@mkdir -p $(GOPATH)/bin
	mv $(BINARY_NAME) $(GOPATH)/bin/$(BINARY_NAME)
	@echo "✓ Installed to $(GOPATH)/bin/$(BINARY_NAME)"
