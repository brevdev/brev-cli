# Development

## Build

`make fast-build` builds a binary for your current machine used for testing/experimentation
`make ci` runs all linters and tests
`make build` runs a full release build (does not release)

## example .env file

```
VERSION=unknown
BREV_API_URL=http://localhost:8080
# BREV_API_URL=<your backend>
```


## adding new commands


https://github.com/spf13/cobra/blob/master/user_guide.md

`pkg/cmd/logout/logout.go` is a minimal command to go off of for adding new commands.

commands for the cli should follow `<VERB>` `<NOUN>` pattern.

Don't forget to add a debug command to `.vscode/launch.json`


### Terminal

- `make` - execute the build pipeline.
- `make help` - print help for the [Make targets](Makefile).

### Visual Studio Code

`F1` → `Tasks: Run Build Task (Ctrl+Shift+B or ⇧⌘B)` to execute the build pipeline.

## Release

make a patch release

```
make version-bump-patch
```

make a minor release

```
make version-bump-minor
```

make a major release
```
make version-bump-major
```

when releasing make sure to


2. [ ] release new version of [workspace-images](https://github.com/brevdev/workspace-images)


## Maintainance

Remember to update Go version in [.github/workflows](.github/workflows), [Makefile](Makefile) and [devcontainer.json](.devcontainer/devcontainer.json).

Notable files:

- [devcontainer.json](.devcontainer/devcontainer.json) - Visual Studio Code Remote Container configuration,
- [.github/workflows](.github/workflows) - GitHub Actions workflows,
- [.github/dependabot.yml](.github/dependabot.yml) - Dependabot configuration,
- [.vscode](.vscode) - Visual Studio Code configuration files,
- [.golangci.yml](.golangci.yml) - golangci-lint configuration,
- [.goreleaser.yml](.goreleaser.yml) - GoReleaser configuration,
- [Dockerfile](Dockerfile) - Dockerfile used by GoReleaser to create a container image,
- [Makefile](Makefile) - Make targets used for development, [CI build](.github/workflows) and [.vscode/tasks.json](.vscode/tasks.json),
- [go.mod](go.mod) - [Go module definition](https://github.com/golang/go/wiki/Modules#gomod),
- [tools.go](tools.go) - [build tools](https://github.com/golang/go/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module).
