.PHONY: build test prove install demo ex2 ex3 clean

BIN_DIR := .bin
GO_ENV := CGO_ENABLED=0 GOCACHE=$(CURDIR)/.cache/go-build

build:
	mkdir -p $(BIN_DIR)
	cd engine && lake build aardengine
	install -m 755 engine/.lake/build/bin/aardengine $(BIN_DIR)/aardengine
	cd runtime && $(GO_ENV) go build -o ../$(BIN_DIR)/aard ./cmd/aard

test:
	cd runtime && $(GO_ENV) go test ./...

prove:
	cd engine && lake build Proofs

install: build

demo: build
	@echo "aard case waits for Lawyer and Council API clients. Use aard run ex1 for a local OpenClaw/Pi run."

ex2: build
	@echo "aard case waits for Lawyer and Council API clients. Use aard run ex2 for a local OpenClaw/Pi run."

ex3: build
	@echo "aard case waits for Lawyer and Council API clients. Use aard run ex3 for a local OpenClaw/Pi run."

clean:
	rm -rf $(BIN_DIR) .cache engine/.lake/build out
