export PATH := $(abspath bin/):${PATH}

OS = $(shell uname | tr A-Z a-z)

user=$(shell id -u -n)
group=$(shell id -g -n)

$( shell mkdir -p bin )

PROJ      = pocket-dex
ORG_PATH  = github.com/Fyve-Labs
REPO_PATH = $(ORG_PATH)/$(PROJ)
VERSION  ?= $(shell ./scripts/git-version)

export GOBIN=$(PWD)/bin
LD_FLAGS="-w -X main.version=$(VERSION)"

##@ Build
build: bin/pocket-dex ## Build binary.

.PHONY: release-binary
release-binary: LD_FLAGS = "-w -X main.version=$(VERSION) -extldflags \"-static\""
release-binary: ## Build release binaries (used to build a final container image).
	@go build -o /go/bin/pocket-dex -v -ldflags $(LD_FLAGS) $(REPO_PATH)

bin/pocket-dex:
	@mkdir -p bin/
	@go install -v -ldflags $(LD_FLAGS) $(REPO_PATH)

go-mod-tidy: ## Run go mod tidy.
	@go mod tidy

.PHONY: lint
lint: ## Run linter.
	@golangci-lint version
	@golangci-lint run

.PHONY: fix
fix: ## Fix lint violations.
	@golangci-lint version
	@golangci-lint fmt

bin/golangci-lint:
	@mkdir -p bin
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | BINARY=golangci-lint bash -s -- latest

##@ Clean
clean: ## Delete all builds and downloaded dependencies.
	@rm -rf bin/


FORMATTING_BEGIN_YELLOW = \033[0;33m
FORMATTING_BEGIN_BLUE = \033[36m
FORMATTING_END = \033[0m

.PHONY: help
help:
	@printf -- "${FORMATTING_BEGIN_BLUE}%s${FORMATTING_END}\n"
	@awk 'BEGIN {\
	    FS = ":.*##"; \
	    printf                "Usage: ${FORMATTING_BEGIN_BLUE}OPTION${FORMATTING_END}=<value> make ${FORMATTING_BEGIN_YELLOW}<target>${FORMATTING_END}\n"\
	  } \
	  /^[a-zA-Z0-9_-]+:.*?##/ { printf "  ${FORMATTING_BEGIN_BLUE}%-46s${FORMATTING_END} %s\n", $$1, $$2 } \
	  /^.?.?##~/              { printf "   %-46s${FORMATTING_BEGIN_YELLOW}%-46s${FORMATTING_END}\n", "", substr($$1, 6) } \
	  /^##@/                  { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
