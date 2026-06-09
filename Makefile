.PHONY: build test test-integration lint clean
build:
	go build -o sparsesvn ./cmd/sparsesvn
test:
	go test ./... -race -count=1
test-integration:
	go test -tags=integration ./test/integration/... -race -count=1 -v
lint:
	go vet ./...
clean:
	rm -f sparsesvn sparsesvn.exe coverage.txt
	rm -rf dist/
