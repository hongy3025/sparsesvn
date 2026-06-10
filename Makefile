.PHONY: build test test-integration lint clean dist

VERSION ?= $(shell git describe --tags --always --dirty 2>nul || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

build:
	go build -ldflags "$(LDFLAGS)" -o sparsesvn.exe ./cmd/sparsesvn

test:
	go test ./... -count=1

test-race:
	go test ./... -race -count=1

test-integration:
	go test -tags=integration ./test/integration/... -race -count=1 -v

lint:
	go vet ./...

clean:
	-if exist sparsesvn.exe del sparsesvn.exe
	-if exist dist rmdir /s /q dist

dist: clean
	mkdir dist
	set GOOS=linux&& set GOARCH=amd64&& go build -ldflags "$(LDFLAGS)" -o dist/sparsesvn-linux-amd64 ./cmd/sparsesvn
	set GOOS=linux&& set GOARCH=arm64&& go build -ldflags "$(LDFLAGS)" -o dist/sparsesvn-linux-arm64 ./cmd/sparsesvn
	set GOOS=darwin&& set GOARCH=amd64&& go build -ldflags "$(LDFLAGS)" -o dist/sparsesvn-darwin-amd64 ./cmd/sparsesvn
	set GOOS=darwin&& set GOARCH=arm64&& go build -ldflags "$(LDFLAGS)" -o dist/sparsesvn-darwin-arm64 ./cmd/sparsesvn
	set GOOS=windows&& set GOARCH=amd64&& go build -ldflags "$(LDFLAGS)" -o dist/sparsesvn-windows-amd64.exe ./cmd/sparsesvn
	set GOOS=windows&& set GOARCH=arm64&& go build -ldflags "$(LDFLAGS)" -o dist/sparsesvn-windows-arm64.exe ./cmd/sparsesvn
	@echo Built: && dir /b dist\
