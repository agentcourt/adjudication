.PHONY: build build-image test prove install demo demo-xproxy demo-quick demo-flash cluster-personas clean

BIN_DIR := .bin
GO_ENV := CGO_ENABLED=0

build: build-image
	mkdir -p $(BIN_DIR)
	cd engine && lake build adcengine
	install -m 755 engine/.lake/build/bin/adcengine $(BIN_DIR)/adcengine
	cd runtime && $(GO_ENV) go build -o ../$(BIN_DIR)/adc ./cmd/adc

build-image:
	../common/pi-container/build-image.sh

test:
	cd runtime && $(GO_ENV) go test ./...

prove:
	cd engine && lake build Proofs

install: build

demo: build
	rm -rf out/ex1-demo
	examples/ex1/sign.sh
	.bin/adc complain --situation examples/ex1/situation.md --out examples/ex1/complaint.md
	.bin/adc case \
		--complaint examples/ex1/complaint.md \
		--out-dir out/ex1-demo \
		--acp-role plaintiff \
		--acp-role defendant \
		--acp-command $(CURDIR)/../common/pi-container/acp-podman.sh

demo-xproxy: build
	rm -rf out/ex1-demo-xproxy
	examples/ex1/sign.sh
	.bin/adc complain --all-through-xproxy --situation examples/ex1/situation.md --out examples/ex1/complaint.md
	.bin/adc case \
		--all-through-xproxy \
		--complaint examples/ex1/complaint.md \
		--out-dir out/ex1-demo-xproxy \
		--acp-role plaintiff \
		--acp-role defendant \
		--acp-command $(CURDIR)/../common/pi-container/acp-podman.sh

demo-quick: build
	rm -rf out/ex1-demo-quick
	examples/ex1/sign.sh
	.bin/adc complain --situation examples/ex1/situation.md --out examples/ex1/complaint.md
	.bin/adc case \
		--complaint examples/ex1/complaint.md \
		--out-dir out/ex1-demo-quick \
		--skip-voir-dire \
		--acp-role plaintiff \
		--acp-role defendant \
		--acp-command $(CURDIR)/../common/pi-container/acp-podman.sh

demo-flash: build
	rm -rf out/ex1-demo-flash
	examples/ex1/sign.sh
	.bin/adc complain --situation examples/ex1/situation.md --out examples/ex1/complaint.md
	.bin/adc case \
		--complaint examples/ex1/complaint.md \
		--out-dir out/ex1-demo-flash \
		--flash gpt-5-mini \
		--acp-role plaintiff \
		--acp-role defendant \
		--acp-command $(CURDIR)/../common/pi-container/acp-podman.sh

cluster-personas: 
	@echo "Start xproxy first in another shell: ./.bin/adc xproxy"
	uv run --script tools/cluster-personas.py --num-personas all --num-samples 3 --num-genes 3 > etc/persona-clusters.csv

pool:
	.bin/adc pool --size 50  | sort | uniq > etc/pool.csv
	wc -l etc/pool.csv

clean:
	rm -rf $(BIN_DIR) engine/.lake/build
