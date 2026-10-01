BIN := flowc
PREFIX ?= $(HOME)/.local

.PHONY: build test vet fmt install clean docs

build:
	go build -o bin/$(BIN) ./cmd/flowc

docs: build
	mkdir -p docs
	./bin/$(BIN) guide > docs/flow-format.md

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

install: build
	install -Dm755 bin/$(BIN) $(PREFIX)/bin/$(BIN)

clean:
	rm -rf bin
