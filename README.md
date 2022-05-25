<boolp align="center">
<img width="230" src="https://raw.githubusercontent.com/brevdev/assets/main/logo.svg"/>
</p>

# Brev.dev

[Brev.dev](https://brev.dev) makes it easy to develop on remote machines. Use Brev.dev to start a project and share your development environment.

## Get Started

https://console.brev.dev

## Docs

https://docs.brev.dev

---
https://user-images.githubusercontent.com/14320477/170173401-01f4f4cd-7099-44b6-a71f-f32cf025a690.mp4


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

1. [ ] run `full-smoke-test` before cutting release to run through some common commands and make sure that they work

2. [ ] release new version of [workspace-images](https://github.com/brevdev/workspace-images)

3. [ ] update [brev's homebrew tap](https://github.com/brevdev/homebrew-brev)


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

## Contributing

Simply create an issue or a pull request.
