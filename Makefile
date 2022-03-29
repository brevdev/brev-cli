.DEFAULT_GOAL := fast-build
VERSION := dev-$(shell git rev-parse HEAD | cut -c 1-8)

.PHONY: fast-build
fast-build: ## go build -o brev
	$(call print-target)
	go build -o brev -ldflags "s -w -X github.com/brevdev/brev-cli/pkg/cmd/version.Version=${VERSION}"

.PHONY: version
version:
	echo ${VERSION}

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
	gofumpt -l -w .

.PHONY: lint
lint: ## golangci-lint
	$(call print-target)
	golangci-lint run --timeout 5m

.PHONY: test
test: ## go test with race detector and code covarage
	$(call print-target)
	go test -race -covermode=atomic -coverprofile=coverage.out ./pkg/...
	go tool cover -html=coverage.out -o coverage.html

.PHONY: test-e2e
test-e2e: ## go test with race detector and code covarage
	$(call print-target)
	go test -race -covermode=atomic -coverprofile=coverage.out ./e2etest/...

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

.PHONY: smoke-test
smoke-test: ## runs `brev version`
	$(call print-target)
	go run main.go --version

.PHONY: full-smoke-test
full-smoke-test: ci fast-build
	# relocate directories used by cli if they exist
	[ ! -d ~/.ssh ] || mv ~/.ssh ~/.ssh.bak
	[ ! -d  ~/.config/Jetbrains ] || mv ~/.config/Jetbrains ~/.config/Jetbrains.bak
	[ ! -d ~/.brev ] || mv ~/.brev ~/.brev.bak

	# cli user flows to smoke test

	# login, set org, list workspaces, start, stop, start, reset, delete & brev jetbrains
	./brev login
	./brev set brev.dev
	./brev ls
	./brev start https://github.com/brevdev/todo-template
	./brev stop brevdev/todo-template
	echo "may have to run this again in a different term"
	./brev start brevdev/todo-template
	./brev reset brevdev/todo-template
	./brev delete brevdev/todo-template
	sleep 5
	./brev jetbrains

	# restore directories used by cli
	[ ! -d ~/.ssh.bak ] || mv ~/.ssh.bak ~/.ssh
	[ ! -d ~/.config/Jetbrains.bak ] || mv ~/.config/Jetbrains.bak ~/.config/Jetbrains
	[ ! -d ~/.brev.bak ] || mv ~/.brev.bak ~/.brev

build-linux-amd:
	GOOS=linux GOARCH=amd64 go build -o brev -ldflags "s -w -X github.com/brevdev/brev-cli/pkg/cmd/version.Version=${VERSION}"

test-e2e-setup-repo: build-linux-amd
	make test-e2e-setup setup_param_path=assets/test_setup_v0_repo.json

test-e2e-setup-norepo: build-linux-amd
	make test-e2e-setup setup_param_path=assets/test_setup_v0_norepo.json

test-e2e-setup:
	# run docker image copy in binary with volume config map + exec setup workspace
	docker kill brev-e2e-test || true
	docker run -d --privileged=true --name brev-e2e-test --rm -it -p 2222:22  brevdev/ubuntu-proxy:0.3.2 bash

	docker exec -it brev-e2e-test mkdir /etc/meta
	docker cp ${setup_param_path} brev-e2e-test:/etc/meta/setup_v0.json

	docker cp brev brev-e2e-test:/usr/local/bin/
	docker exec -it brev-e2e-test /usr/local/bin/brev setupworkspace

	# validate container is in proper state
	docker exec -it brev-e2e-test /usr/local/bin/brev validateworkspacesetup

shell-into-e2e:
	docker exec -it brev-e2e-test bash