INSTALL_DIR := $(HOME)/bin

.PHONY: build install test vet clean

build:
	go build -o bin/forza-radio ./cmd/forza-radio
	go build -o bin/forza-mock ./cmd/forza-mock

## install builds the binaries and copies them and the launcher to $(INSTALL_DIR) (must be on PATH).
install: build
	mkdir -p $(INSTALL_DIR)
	install -m 755 bin/forza-radio $(INSTALL_DIR)/forza-radio
	install -m 755 bin/forza-mock $(INSTALL_DIR)/forza-mock
	install -m 755 scripts/forza-radio-launcher.sh $(INSTALL_DIR)/forza-radio-launcher.sh
	@echo "Installed forza-radio, forza-mock, and forza-radio-launcher.sh to $(INSTALL_DIR)"

test:
	go test ./...

vet:
	go vet ./...

clean:
	rm -rf bin
