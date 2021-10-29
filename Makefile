.PHONY: test cover build all clean parallelizable-steps

SHELL := /bin/bash
GOPATH ?= $(shell go env GOPATH)
PATH := $(GOPATH)/bin:$(PATH)

PACKAGES := utils database testutils monitor
LINT_PACKAGES = $(PACKAGES) gonittest log cmd
VET_PACKAGES = $(PACKAGES) gonittest log cmd

fmtcheck = @if goimports -l $(1) | read var; then echo "goimports check failed for $(1):\n `goimports -d $(1)`"; exit 1; fi

ifeq ($(VERBOSE), 1)
	GO_TEST_ARGS := -v
else
	GO_TEST_ARGS := 
endif

GO_TEST := @go test $(GO_TEST_ARGS)
GO_COVER := @go tool cover
BUILD_DIR := ./.build
DIST_DIR := ./dist/gonit
VENDOR := vendor
BUILD_DATE := $(shell date -u '+%Y-%m-%d %I:%M:%S UTC' 2> /dev/null)
GIT_HASH := $(shell git rev-parse HEAD 2> /dev/null)
GO_BUILD := go build -ldflags "-X 'main.buildDate=$(BUILD_DATE)' -X main.commit=$(GIT_HASH) -s -w"
GO_MOD := @go mod
TOOL_PATH ?= $(DIST_DIR)/$(TOOL_NAME)

# This allows forcing the dependencies to finish installing even in parallel mode
all:
	@$(MAKE) get-build-deps
	@$(MAKE) download
	@$(MAKE) -s validate-command-gocovmerge validate-command-golint
	@$(MAKE) -s parallelizable-steps

parallelizable-steps: vet lint build test race-test cover

download:
	$(GO_MOD) download

build/%:
	@echo "+ Building GOARCH=$(*F)"
	@mkdir -p $(DIST_DIR)
	@GOARCH=$(*F) $(GO_BUILD) -o $(TOOL_PATH) .
	@echo "*** Gonit binary created under $(TOOL_PATH) ***"

build: build/amd64

clean:
	-rm -rf $(BUILD_DIR)
	-rm -rf $(DIST_DIR)

get-build-deps:
	@echo "+ Downloading build dependencies"
	@go get golang.org/x/tools/cmd/goimports
	@go get golang.org/x/lint/golint
	@go get github.com/wadey/gocovmerge


validate-command-%:
	@which $(*F) > /dev/null || (echo "Tool $(*F) must be in your PATH" 2>&1 && exit 1)

vet:  $(addprefix vet-, $(VET_PACKAGES))
	@go vet .

vet-%:
	@echo "+ $@"
	@go vet ./$(*F)

lint: $(addprefix lint-, $(LINT_PACKAGES))
	@echo "+ $@"
	@$(MAKE) -s validate-command-golint
	@golint .
	$(call fmtcheck, .)

lint-%:
	@echo "+ $@"
	@golint ./$(*F)
	$(call fmtcheck, $(*F))


test: $(addprefix test-, $(PACKAGES))
	@echo "+ Testing gonit tool"
	@go test .

race-test: $(addprefix race-test-, $(PACKAGES))


cover-%: $(BUILD_DIR)/%.coverprofile
	@echo "+ $@"
	$(GO_COVER) -html=$^ -o=$(BUILD_DIR)/$(*F).html

cover: test $(addprefix cover-, $(PACKAGES))
	@$(MAKE) -s validate-command-gocovmerge
	@gocovmerge $(wildcard $(BUILD_DIR)/*.coverprofile) > $(BUILD_DIR)/cover.out
	$(GO_COVER) -html=$(BUILD_DIR)/cover.out -o=$(BUILD_DIR)/coverage.html

test-%:
	@echo "+ Testing $(*F)"
	@mkdir -p $(BUILD_DIR)
	$(GO_TEST) -covermode=count -coverprofile=$(BUILD_DIR)/$(*F).coverprofile ./$(*F)

race-test-%:
	@echo "+ Testing $(*F) (race conditions)"
	@mkdir -p $(BUILD_DIR)
	$(GO_TEST) -race ./$(*F)

