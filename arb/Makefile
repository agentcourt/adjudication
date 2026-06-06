.PHONY: build test prove install demo ex02 ex03 ex04 ex05 ex06 clean

BIN_DIR := .bin
GO_ENV := CGO_ENABLED=0 GOCACHE=$(CURDIR)/.cache/go-build

build:
	mkdir -p $(BIN_DIR)
	cd engine && lake build aarengine
	install -m 755 engine/.lake/build/bin/aarengine $(BIN_DIR)/aarengine
	cd runtime && $(GO_ENV) go build -o ../$(BIN_DIR)/aar ./cmd/aar

test:
	cd runtime && $(GO_ENV) go test ./...

prove:
	cd engine && lake build Proofs

install: build

demo: build
	@echo "aar case waits for Lawyer API clients. See manual.md."

ex02: build
	@echo "aar case waits for Lawyer API clients. See manual.md."

ex03: build
	@echo "aar case waits for Lawyer API clients. See manual.md."

ex04: build
	@echo "aar case waits for Lawyer API clients. See manual.md."

ex05: build
	@echo "aar case waits for Lawyer API clients. See manual.md."

ex06: build
	@echo "aar case waits for Lawyer API clients. See manual.md."

clean:
	rm -rf $(BIN_DIR) engine/.lake/build out/ex01-demo out/ex02-demo out/ex03-demo out/ex04-demo out/ex05-demo out/ex06-demo
