.PHONY: build test test-integration lint clean dist

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

build:
	go build -ldflags "$(LDFLAGS)" -o sparsesvn$(if $(filter Windows_NT,$(OS)),.exe) ./cmd/sparsesvn

test:
	go test ./... -count=1

test-race:
	go test ./... -race -count=1

test-integration:
	go test -tags=integration ./test/integration/... -race -count=1 -v

lint:
	go vet ./...

clean:
	rm -rf sparsesvn sparsesvn.exe dist/

dist: clean
	mkdir -p dist
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/sparsesvn-linux-amd64 ./cmd/sparsesvn
	GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/sparsesvn-linux-arm64 ./cmd/sparsesvn
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/sparsesvn-darwin-amd64 ./cmd/sparsesvn
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/sparsesvn-darwin-arm64 ./cmd/sparsesvn
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/sparsesvn-windows-amd64.exe ./cmd/sparsesvn
	GOOS=windows GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/sparsesvn-windows-arm64.exe ./cmd/sparsesvn
	@echo "Built:" && ls dist/
