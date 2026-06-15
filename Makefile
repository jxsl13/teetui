# teetui — convenience targets.
# All builds force CGO_ENABLED=0 for static, dependency-free binaries.
export CGO_ENABLED = 0

BINARY := teetui

.PHONY: build test vet release-snapshot clean

## build: compile the teetui binary for the host platform
build:
	go build -trimpath -ldflags '-s -w' -o $(BINARY) .

## vet: run go vet on all packages
vet:
	go vet ./...

## test: run the test suite
test:
	go test ./...

## release-snapshot: build the full cross-platform release locally (no publish)
release-snapshot:
	goreleaser release --snapshot --clean

## clean: remove build artifacts
clean:
	rm -rf $(BINARY) dist/
