.PHONY: all

.DEFAULT_GOAL := help

ROOT_DIR := $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
GOBIN := ${ROOT_DIR}/bin

PROJECT_ROOT:=$(shell git rev-parse --show-toplevel)
COMMIT := $(shell git rev-parse --short HEAD)
BRANCH := $(shell git rev-parse --abbrev-ref HEAD)
REPO := $(shell basename `git rev-parse --show-toplevel`)
DATE := $(shell date +%Y-%m-%d-%H-%M-%S)
APP_NAME := $(shell basename `git rev-parse --show-toplevel`)

export GOBIN

MAKE_LIB:=$(PROJECT_ROOT)/scripts
-include $(MAKE_LIB)/tests.mk
-include $(MAKE_LIB)/lints.mk
-include $(MAKE_LIB)/tools.mk
-include $(MAKE_LIB)/generator.mk


GO_BUILD_FLAGS=-ldflags="-X 'github.com/kaikenlabs/tag/internal/commands.version=dev-$(BRANCH)-$(COMMIT)'"

#####################
##@ Main   
#####################

ci: log lint scan test ## Run ci tasks

build: ## build go files
	go build $(GO_BUILD_FLAGS) -o $(APP_NAME)

# HELP
# This will output the help for each task
# thanks to https://marmelab.com/blog/2016/02/29/auto-documented-makefile.html
help: ## This help.
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)



#####################
#  Private Targets  #
#####################

log: # log env vars
	@echo "\n"
	@echo "COMMIT               $(COMMIT)"
	@echo "BRANCH               $(BRANCH)"
	@echo "APP_NAME             $(APP_NAME)"
	@echo "DATE                 $(DATE)"
	@echo "\n"
