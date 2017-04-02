.PHONY: all test install-deps test-ci

all: build-local

test: install-deps fmt lint vet
	@echo "+ $@"
	@go test -race -v

test-ci:
	@echo "+ $@"
	make test
	make build-local
	@./run-integration-tests.sh

build-local:
	@mkdir -p "bin"
	go build -o "bin/libnetwork-ovn-plugin" ./

install-deps:
	@echo "+ $@"
	@go get -u github.com/golang/lint/golint
	@go get -u github.com/tools/godep
	@go get -u golang.org/x/tools/cmd/cover
	@go get -u github.com/mattn/goveralls
	@go get -d ./...

lint:
	@echo "+ $@"
	@test -z "$$(golint ./... | grep -v vendor/ | tee /dev/stderr)"

fmt:
	@echo "+ $@"
	@test -z "$$(gofmt -s -l . | grep -v vendor/ | tee /dev/stderr)"

vet:
	@echo "+ $@"
	@go vet ./ ./ovn

clean:
	@if [ -d bin ]; then \
		echo "Removing binaries"; \
		rm -rf bin; \
	fi
