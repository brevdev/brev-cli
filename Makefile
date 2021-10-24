BIN_NAME?=brev
BIN_VERSION?=0.1.6

GOCMD=GO

GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOFMT=$(GOCMD) fmt
GOCLEAN=$(GOCMD) clean

PATH_BIN=bin
PATH_DIST=dist
PATH_PROJECT=github.com/brevdev/brev-cli
PATH_MAIN=$(PATH_PROJECT)/pkg/cmd

BUILDCMD=$(GOBUILD) -ldflags "-X $(FIELD_VERSION)=$(BIN_VERSION)"

build: linux darwin

linux:
	env GOOS=linux GOARCH=amd64 $(BUILDCMD) \
			-o $(PATH_BIN)/nix/$(BIN_NAME) -v $(PATH_MAIN)

darwin:
	env GOOS=darwin GOARCH=amd64 $(BUILDCMD) \
			-o $(PATH_BIN)/osx/$(BIN_NAME) -v $(PATH_MAIN)

darwin-homebrew:
	env GOOS=darwin GOARCH=arm64 $(BUILDCMD) \
			-o $(PATH_BIN)/homebrew/$(BIN_NAME)-arm64_big_sur -v $(PATH_MAIN)
	env CGO_CFLAGS="-mmacosx-version-min=11.2" CGO_LDFLAGS="-mmacosx-version-min=11.2" GOOS=darwin GOARCH=amd64 $(BUILDCMD) \
			-o $(PATH_BIN)/homebrew/$(BIN_NAME)-big_sur -v $(PATH_MAIN)
	env CGO_CFLAGS="-mmacosx-version-min=10.15" CGO_LDFLAGS="-mmacosx-version-min=10.15" GOOS=darwin GOARCH=amd64 $(BUILDCMD) \
			-o $(PATH_BIN)/homebrew/$(BIN_NAME)-catalina -v $(PATH_MAIN)
	env CGO_CFLAGS="-mmacosx-version-min=10.14" CGO_LDFLAGS="-mmacosx-version-min=10.14" GOOS=darwin GOARCH=amd64 $(BUILDCMD) \
			-o $(PATH_BIN)/homebrew/$(BIN_NAME)-mojave -v $(PATH_MAIN)

dist-homebrew: darwin-homebrew
	mkdir -p $(PATH_DIST)/homebrew
	tar -C $(PATH_BIN)/homebrew/ -czf $(PATH_DIST)/homebrew/brev-homebrew-bundle.tar.gz .
	shasum -a 256 $(PATH_DIST)/homebrew/brev-homebrew-bundle.tar.gz | awk '{print $$1}' > $(PATH_DIST)/homebrew/brev-homebrew-bundle.tar.gz.sha256
	@echo "\nsha256:"
	@cat $(PATH_DIST)/homebrew/brev-homebrew-bundle.tar.gz.sha256

# EVERYTHING ABOVE IS FROM THE OLD MAKEFILE

.DEFAULT_GOAL := dev

.PHONY: dev
dev: ## dev build
dev: clean install generate vet fmt lint test mod-tidy

.PHONY: ci
ci: ## CI build
ci: dev diff

.PHONY: clean
clean: ## remove files created during build pipeline
	$(call print-target)
	rm -rf dist
	rm -f coverage.*

.PHONY: install
install: ## go install tools
	$(call print-target)
	cd tools && go install $(shell cd tools && go list -f '{{ join .Imports " " }}' -tags=tools)

.PHONY: generate
generate: ## go generate
	$(call print-target)
	go generate ./...

.PHONY: vet
vet: ## go vet
	$(call print-target)
	go vet ./...

.PHONY: fmt
fmt: ## go fmt
	$(call print-target)
	go fmt ./...

.PHONY: lint
lint: ## golangci-lint
	$(call print-target)
	golangci-lint run

.PHONY: test
test: ## go test with race detector and code covarage
	$(call print-target)
	go test -race -covermode=atomic -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

.PHONY: mod-tidy
mod-tidy: ## go mod tidy
	$(call print-target)
	go mod tidy
	cd tools && go mod tidy

.PHONY: diff
diff: ## git diff
	$(call print-target)
	git diff --exit-code
	RES=$$(git status --porcelain) ; if [ -n "$$RES" ]; then echo $$RES && exit 1 ; fi

.PHONY: build
build: ## goreleaser --snapshot --skip-publish --rm-dist
build: install
	$(call print-target)
	goreleaser --snapshot --skip-publish --rm-dist

fast-build:
	go build -o brev 

.PHONY: release
release: ## goreleaser --rm-dist
release: install
	$(call print-target)
	goreleaser --rm-dist

.PHONY: run
run: ## go run
	@go run -race .

.PHONY: go-clean
go-clean: ## go clean build, test and modules caches
	$(call print-target)
	go clean -r -i -cache -testcache -modcache

.PHONY: help
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

define print-target
    @printf "Executing target: \033[36m$@\033[0m\n"
endef
