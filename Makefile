.PHONY: test cover build all clean parallelizable-steps

PACKAGES := utils database testutils monitor
LINT_PACKAGES = $(PACKAGES) gonittest
VET_PACKAGES = $(PACKAGES) gonittest

ifeq ($(VERBOSE), 1)
	GO_TEST_ARGS := -v
else
	GO_TEST_ARGS := 
endif

GO_TEST := @go test -covermode=count $(GO_TEST_ARGS)
GO_COVER := @go tool cover
BUILD_DIR := ./.build
DIST_DIR := ./dist/gonit
VENDOR := vendor

# This allows forcing the dependencies to finish installing even in parallel mode
all: get-build-deps
	@$(MAKE) -s validate-command-gocovmerge validate-command-golint
	@$(MAKE) -s parallelizable-steps

parallelizable-steps: vet lint build test cover

build:
	@echo "+ $@"
	@mkdir -p $(DIST_DIR)
	@go build -o $(DIST_DIR)/gonit .
	@strip $(DIST_DIR)/gonit
	@echo "*** Gonit binary created under $(DIST_DIR)/gonit ***"

clean:
	-rm -rf $(BUILD_DIR)
	-rm -rf $(DIST_DIR)

get-build-deps:
	@echo "+ Downloading build dependencies"
	@go get golang.org/x/tools/cmd/goimports
	@go get github.com/tools/godep
	@go get github.com/golang/lint/golint
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

lint-%:
	@echo "+ $@"
	@golint ./$(*F)

test: $(addprefix test-, $(PACKAGES))
	@echo "+ Testing gonit tool"
	@go test .

cover: test $(addprefix cover-, $(PACKAGES))
	@$(MAKE) -s validate-command-gocovmerge
	@gocovmerge $(wildcard $(BUILD_DIR)/*.coverprofile) > $(BUILD_DIR)/cover.out
	$(GO_COVER) -html=$(BUILD_DIR)/cover.out -o=$(BUILD_DIR)/coverage.html


cover-%: $(BUILD_DIR)/%.coverprofile
	@echo "+ $@"
	$(GO_COVER) -html=$^ -o=$(BUILD_DIR)/$(*F).html


test-%:
	@echo "+ Testing $(*F)"
	@mkdir -p $(BUILD_DIR)
	$(GO_TEST)  -coverprofile=$(BUILD_DIR)/$(*F).coverprofile ./$(*F)

