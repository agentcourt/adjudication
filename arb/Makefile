.PHONY: build test prove install demo ex2 ex3 ex4 ex5 ex6 clean

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
	@echo "aar case waits for Lawyer API clients. See ../lawyerapi.md and ../clawyers-ex1.md."

ex2: build
	@echo "aar case waits for Lawyer API clients. See ../lawyerapi.md."

ex3: build
	@echo "aar case waits for Lawyer API clients. See ../lawyerapi.md."

ex4: build
	@echo "aar case waits for Lawyer API clients. See ../lawyerapi.md."

ex5: build
	@echo "aar case waits for Lawyer API clients. See ../lawyerapi.md."

ex6: build
	@echo "aar case waits for Lawyer API clients. See ../lawyerapi.md."

clean:
	rm -rf $(BIN_DIR) engine/.lake/build out/ex1-demo out/ex2-demo out/ex3-demo out/ex4-demo out/ex5-demo out/ex6-demo
