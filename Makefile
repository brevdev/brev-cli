.DEFAULT_GOAL := fast-build
VERSION := dev-$(shell git rev-parse HEAD | cut -c 1-8)

.PHONY: fast-build
fast-build: ## go build -o brev
	$(call print-target)
	echo ${VERSION}
	CGO_ENABLED=0 go build -o brev -ldflags "-X github.com/brevdev/brev-cli/pkg/cmd/version.Version=${VERSION}"

.PHONY: install-dev
install-dev: fast-build ## go install
	cp brev $(shell go env GOPATH)/bin/

.PHONY: version
version:
	echo ${VERSION}

.PHONY: dev
dev: ## dev build
dev: clean install-tools generate vet fmt lint test mod-tidy

.PHONY: ci
ci: ## CI build
ci: dev diff

.PHONY: install
install: dep-tools dep-python-tools ## go install tools


.PHONY: dep-tools
dep-tools: ## go install tools
	$(call print-target)
	cd tools && go install $(shell cd tools && go list -e -f '{{ join .Imports " " }}' -tags=tools) && cd -

.PHONY: dep-python-tools
dep-python-tools: ## install python tools
	pip install --upgrade Pygments
	pip install jupyter==1.0.0


.PHONY: clean
clean: ## remove files created during build pipeline
	$(call print-target)
	rm -rf dist
	rm -f coverage.*

.PHONY: install-tools
install-tools: ## go install tools
	$(call print-target)
	cd tools && go install $(shell cd tools && go list -e -f '{{ join .Imports " " }}' -tags=tools)

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

fmtcheck: ## go fmt --check
	$(call print-target)
	# gofumpt check
	gofumpt -l -d -e .

.PHONY: lint
lint: ## golangci-lint
	$(call print-target)
	golangci-lint run --timeout 10m0s
	workflowcheck -show-pos ./...
	buf lint

.PHONY: test
test: ## go test with race detector and code covarage
	$(call print-target)
	go test -race -covermode=atomic -coverprofile=coverage.out ./pkg/...
	go tool cover -html=coverage.out -o coverage.html

.PHONY: test-e2e
test-e2e: ## go test with race detector and code covarage
	$(call print-target)
	go test -timeout 90m -race -covermode=atomic -coverprofile=coverage.out ./e2etest/...

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
build: install-tools
	$(call print-target)
	goreleaser --snapshot --skip-publish --rm-dist

.PHONY: release
release: ## goreleaser --rm-dist
release: install-tools
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

.PHONY: build-linux-amd
build-linux-amd:
	$(call print-target)
	echo ${VERSION}
	GOOS=linux GOARCH=amd64 go build -o brev -ldflags "-X github.com/brevdev/brev-cli/pkg/cmd/version.Version=${VERSION}"

.PHONY: build-darwin-amd
build-darwin-amd:
	$(call print-target)
	echo ${VERSION}
	GOOS=darwin GOARCH=amd64 go build -o brev -ldflags "-X github.com/brevdev/brev-cli/pkg/cmd/version.Version=${VERSION}"


.PHONY: setup-workspace-repo
setup-workspace-repo: build-linux-amd
	make setup-workspace setup_param_path=assets/test_setup_v0_repo.json

.PHONY: setup-workspace-norepo
setup-workspace-norepo: build-linux-amd
	make setup-workspace setup_param_path=assets/test_setup_v0_norepo.json

.PHONY: setup-workspace-blank
setup-workspace-blank:
	make setup-workspace setup_param_path=assets/blank_v0.json

container_name=setup-workspace
image_name=test-workspace

.PHONY: build-test-workspace
build-test-workspace:
	cd workspacedocker && docker build -t $(image_name) . && cd -

.PHONY: setup-workspace
setup-workspace: build-linux-amd build-test-workspace
	# run docker image copy in binary with volume config map + exec setup workspace
	[ "${setup_param_path}" ] || ( echo "'setup_param_path' not provided"; exit 1 )
	make time
	docker kill $(container_name) || true
	docker run -d --privileged=true --name $(container_name) --rm -it -p 22776:22778 -p 2222:22  brevdev/ubuntu-proxy:0.3.13 zsh

	docker exec -it $(container_name) mkdir /etc/meta
	docker cp ${setup_param_path} $(container_name):/etc/meta/setup_v0.json

	docker cp brev $(container_name):/usr/local/bin/
	make time
	docker exec -it $(container_name) /usr/local/bin/brev setupworkspace
	make time


.PHONY: workspace-dev-script
workspace-dev-script: fast-build clean-simulated-workspace
	make simulate-workspace setup_param_path=assets/std_setup_v0.json
	echo "exit the shell and re-run to reset workspace"
	make shell-into-workspace

.PHONY: simulate-workspace
simulate-workspace:
	[ "${setup_param_path}" ] || ( echo "'setup_param_path' not provided"; exit 1 )
	make time
	docker kill $(container_name) || true
	echo "modify workspace files in devworkspace"
	docker run -d --privileged=true --name $(container_name) --rm -it -p 2222:22 -v $(shell pwd)/devworkspace:/home/brev/workspace brevdev/ubuntu-proxy:0.3.13 zsh

	docker exec -it $(container_name) mkdir /etc/meta
	docker cp ${setup_param_path} $(container_name):/etc/meta/setup_v0.json

	# remove when released binary has setupworkspace
	docker cp brev $(container_name):/usr/local/bin/
	make time
	docker exec -it $(container_name) /usr/local/bin/brev setupworkspace
	make time

.PHONY:clean-simulated-workspace
clean-simulated-workspace:
	sudo rm -rf devworkspace

.PHONY: shell-into-workspace
shell-into-workspace:
	docker exec --user brev -it $(container_name) zsh --login

time:
	date +"%FT%T%z"

get-latest-tag:
	git describe --tags --abbrev=0

version-bump-patch:
	make version-bump type=patch

version-bump-minor:
	make version-bump type=minor

version-bump-major:
	make version-bump type=major

fetch-tags:
	git fetch --all --tags

version-bump: fetch-tags
	[ "${type}" ] || ( echo "'type' not provided [patch, minor, major]"; exit 1 )
	bump2version --current-version $(shell git describe --tags --abbrev=0) ${type} --list --tag --serialize v{major}.{minor}.{patch} --tag-name {new_version}  | grep new_version | sed -r s,"^.*=",, | xargs git push origin


lr := $(shell git rev-parse latest-review)
cr := $(shell git rev-parse main)

review:
	git diff ${lr}...${cr}

review-github:
	open https://github.com/brevdev/brev-cli/compare/${lr}...${cr}

review_tag := review-`date +"%F-%H-%M"`
review-mark-done:
	git tag latest-review -f
	git tag -a ${review_tag}
	git push origin latest-review --force
	git push origin ${review_tag}

gen-e2e:
	cat e2etest/setup/setup_test.go|  grep -Eo "Test_\w+" | xargs python bin/gen-e2e-actions.py

## removed queued jobs from github actions
remove-queued-jobs:
	./bin/remove-queued-jobs.sh

new-cmd:
	mkdir -p pkg/cmd/${name}
	touch pkg/cmd/${name}/${name}.go
	# populate the template
	./bin/newcmd.sh ${name} > pkg/cmd/${name}/${name}.go

.PHONY: develop-with-nix
develop-with-nix:
	nix develop .
