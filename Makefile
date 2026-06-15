# teetui — convenience targets.
# All builds force CGO_ENABLED=0 for static, dependency-free binaries.
export CGO_ENABLED = 0

BINARY := teetui

.PHONY: build test vet e2e release-snapshot clean

## build: compile the teetui binary for the host platform
build:
	go build -trimpath -ldflags '-s -w' -o $(BINARY) .

## vet: run go vet on all packages
vet:
	go vet ./...

## test: run the test suite
test:
	go test ./...

## e2e: bring up the compose harness and run the gated live suite IN-NETWORK
# Mirrors the CI `e2e` job (SPEC T49/C14): servers are addressed by compose
# service name, so the test runs inside a container on the compose network.
e2e:
	docker compose -p teetui-e2e -f e2e/docker-compose.yml up -d --build
	docker run --rm --network teetui-e2e_default -v "$(PWD)":/src -w /src \
		-e TW_E2E=1 \
		-e TW_E2E_DDNET_06=ddnet:8303 \
		-e TW_E2E_DDNET_07=ddnet:8303 \
		-e TW_E2E_VANILLA_07=teeworlds7:8303 \
		golang:1.26 go test -tags e2e -count=1 -timeout 10m ./e2e/... -v; \
		status=$$?; \
		docker compose -p teetui-e2e -f e2e/docker-compose.yml down; \
		exit $$status

## release-snapshot: build the full cross-platform release locally (no publish)
release-snapshot:
	goreleaser release --snapshot --clean

## clean: remove build artifacts
clean:
	rm -rf $(BINARY) dist/
