.PHONY: build build-image test prove demo ex2 clean

BIN_DIR := .bin
GO_ENV := CGO_ENABLED=0 GOCACHE=$(CURDIR)/.cache/go-build
ATTORNEY_MODEL := openai://gpt-5

build: build-image
	mkdir -p $(BIN_DIR)
	cd engine && lake build aardengine
	install -m 755 engine/.lake/build/bin/aardengine $(BIN_DIR)/aardengine
	cd runtime && $(GO_ENV) go build -o ../$(BIN_DIR)/aard ./cmd/aard

build-image:
	../common/pi-container/build-image.sh

test:
	cd runtime && $(GO_ENV) go test ./...

prove:
	cd engine && lake build Proofs

demo: build
	rm -rf out/ex1-demo
	.bin/aard complain --situation examples/ex1/situation.md --out examples/ex1/complaint.md
	.bin/aard case --complaint examples/ex1/complaint.md --out-dir out/ex1-demo --attorney-model $(ATTORNEY_MODEL)

ex2: build
	rm -rf out/ex2-demo
	.bin/aard complain --situation examples/ex2/situation.md --out examples/ex2/complaint.md
	.bin/aard case --complaint examples/ex2/complaint.md --out-dir out/ex2-demo --attorney-model $(ATTORNEY_MODEL)

clean:
	rm -rf $(BIN_DIR) .cache engine/.lake/build out
