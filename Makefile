.PHONY: all

.DEFAULT_GOAL := help

PROJECT_ROOT := $(shell git rev-parse --show-toplevel)
COMMIT := $(shell git rev-parse --short HEAD)
APP_NAME := $(shell basename `git rev-parse --show-toplevel`)

MISE := mise exec --

export GOTOOLCHAIN := local

MAKE_LIB := $(PROJECT_ROOT)/scripts
-include $(MAKE_LIB)/tests.mk
-include $(MAKE_LIB)/lints.mk
-include $(MAKE_LIB)/generator.mk

GO_BUILD_FLAGS=-ldflags="-X 'main.Version=dev-$(COMMIT)'"

#####################
##@ Main
#####################

build: ## build go files
	@$(MISE) go build $(GO_BUILD_FLAGS) -o $(APP_NAME)

install: build ## build and install to ~/.local/bin
	@mkdir -p ~/.local/bin
	@cp $(APP_NAME) ~/.local/bin/$(APP_NAME)
	@echo "Installed $(APP_NAME) to ~/.local/bin/$(APP_NAME)"

tools: ## Install the pinned toolchain from mise.toml
	@GOTOOLCHAIN=auto mise install

clean: ## Remove build artifacts and coverage reports
	@rm -f $(APP_NAME) coverage.out coverage.html

help: ## This help.
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
