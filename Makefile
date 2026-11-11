BIN     := dscli
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
PLATFORMS := linux/amd64 linux/386 windows/amd64 darwin/amd64

.PHONY: all build test vet cross clean fmt tidy

all: test build

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/$(BIN)_$(shell go env GOOS)_$(shell go env GOARCH) ./cmd/dscli

test:
	go test ./...

vet:
	go vet ./...

cross: clean
	@mkdir -p dist
	@for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; ext=""; \
		[ "$$os" = "windows" ] && ext=".exe"; \
		echo "build $$os/$$arch"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags "$(LDFLAGS)" \
			-o dist/$(BIN)_$${os}_$${arch}$$ext ./cmd/dscli || exit 1; \
	done
	@cd dist && sha256sum * > SHA256SUMS && cat SHA256SUMS

clean:
	rm -rf dist

fmt:
	gofmt -l -w .

tidy:
	go mod tidy
