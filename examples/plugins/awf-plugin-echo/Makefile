.PHONY: build test lint install-local uninstall-local package checksums

PLUGIN_NAME := awf-plugin-echo
VERSION := 1.0.0
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
DIST_DIR := dist
PACKAGE_DIR := package
ARCHIVE := $(PLUGIN_NAME)_$(VERSION)_$(GOOS)_$(GOARCH).tar.gz

build:
	mkdir -p "$(DIST_DIR)"
	go build -o $(DIST_DIR)/$(PLUGIN_NAME) .

test:
	go test .

lint:
	go vet .

install-local: build
	mkdir -p "$${HOME}/.local/share/awf/plugins/echo"
	cp "$(DIST_DIR)/$(PLUGIN_NAME)" "$${HOME}/.local/share/awf/plugins/echo/$(PLUGIN_NAME)"
	cp plugin.yaml "$${HOME}/.local/share/awf/plugins/echo/plugin.yaml"

uninstall-local:
	rm -f "$${HOME}/.local/share/awf/plugins/echo/$(PLUGIN_NAME)"
	rm -f "$${HOME}/.local/share/awf/plugins/echo/plugin.yaml"

package: build
	mkdir -p "$(DIST_DIR)/$(PACKAGE_DIR)"
	find "$(DIST_DIR)/$(PACKAGE_DIR)" -mindepth 1 -maxdepth 1 -exec rm -r -- {} +
	cp "$(DIST_DIR)/$(PLUGIN_NAME)" plugin.yaml "$(DIST_DIR)/$(PACKAGE_DIR)/"
	tar -C "$(DIST_DIR)/$(PACKAGE_DIR)" -czf "$(DIST_DIR)/$(ARCHIVE)" .

checksums: package
	cd "$(DIST_DIR)" && sha256sum *.tar.gz > checksums.txt
