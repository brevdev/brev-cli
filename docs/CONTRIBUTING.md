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
## running a command against a brev-deploy workspace
```
make && BREV_API_URL=`brev ls | grep brev-deploy | awk '{ print "https://8080-"$3 }'` ./brev start https://gitlab.com/reedrichards/hn
```

## adding new commands

create a directory in `pkg/cmd` for your command, a go file, and documentation
file

```
mkdir pkg/cmd/recreate/
touch pkg/cmd/recreate/recreate.go
touch pkg/cmd/recreate/doc.md
```

add the following template to `recreate.go`

```go
// Package recreate is for the recreate command
package recreate

import (
	_ "embed"

	"github.com/spf13/cobra"

	"github.com/brevdev/brev-cli/pkg/cmd/cmderrors"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	stripmd "github.com/writeas/go-strip-markdown"
)

//go:embed doc.md
var long string

type reCreateStore interface{}

func NewCmdRecreate(t *terminal.terminal, store reCreateStore) *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "recreate",
		DisableFlagsInUseLine: true,
		Short:                 "TODO",
		Long:                  stripmd.Strip(long),
		Example:               "TODO",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := RunReCreate(t, args, store)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}
	return cmd
}

func RunReCreate(_ *terminal.terminal,_ []string, _ reCreateStore) error {
	return nil
}

```

Implement `RunReCreate`

```go
// Package recreate is for the recreate command
package recreate

import (
	_ "embed"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/brevdev/brev-cli/pkg/cmd/cmderrors"
	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/cmd/util"
	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/featureflag"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

//go:embed doc.md
var long string

type recreateStore interface {
	completions.CompletionStore
	util.GetWorkspaceByNameOrIDErrStore
	ResetWorkspace(workspaceID string) (*entity.Workspace, error)
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetCurrentUser() (*entity.User, error)
	CreateWorkspace(organizationID string, options *store.CreateWorkspacesOptions) (*entity.Workspace, error)
	GetWorkspace(id string) (*entity.Workspace, error)
	DeleteWorkspace(workspaceID string) (*entity.Workspace, error)
}

func NewCmdRecreate(t *terminal.Terminal, store recreateStore) *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "recreate",
		DisableFlagsInUseLine: true,
		Short:                 "TODO",
		Long:                  "TODO",
		Example:               "TODO",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := RunRecreate(t, args, store)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}
	return cmd
}

func RunRecreate(t *terminal.terminal, args []string, recreateStore recreateStore) error {
	for _, arg := range args {
		err := hardResetProcess(arg, t, recreateStore)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}
	return nil
}
// ...
```

add command to `pkg/cmd/cmd.go`

```diff
diff --git a/pkg/cmd/cmd.go b/pkg/cmd/cmd.go
index a33540c..b03d5f2 100644
--- a/pkg/cmd/cmd.go
+++ b/pkg/cmd/cmd.go
@@ -23,6 +23,7 @@ import (
        "github.com/brevdev/brev-cli/pkg/cmd/portforward"
        "github.com/brevdev/brev-cli/pkg/cmd/profile"
        "github.com/brevdev/brev-cli/pkg/cmd/proxy"
+       "github.com/brevdev/brev-cli/pkg/cmd/recreate"
        "github.com/brevdev/brev-cli/pkg/cmd/refresh"
        "github.com/brevdev/brev-cli/pkg/cmd/reset"
        "github.com/brevdev/brev-cli/pkg/cmd/runtasks"
@@ -243,6 +244,7 @@ func createCmdTree(cmd *cobra.Command, t *terminal.Terminal, loginCmdStore *stor
        cmd.AddCommand(healthcheck.NewCmdHealthcheck(t, noLoginCmdStore))

        cmd.AddCommand(setupworkspace.NewCmdSetupWorkspace(noLoginCmdStore))
+       cmd.AddCommand(recreate.NewCmdRecreate(t, loginCmdStore))
 }

 func hasHousekeepingCommands(cmd *cobra.Command) bool {
```

test your function

```
make && ./brev recreate
```

add documentation by editing `pkg/cmd/recreate/doc.md`. Docs should fill out the
minimum fields:

```
<!-- Insert title here -->
#
## SYNOPSIS

## DESCRIPTION

## EXAMPLE

## SEE ALSO

```

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

## e2e tests

generate workflows for github actions  

```
make gen-e2e
```

### configure a runner fo e2e tests 

TODO:
  - configure workspace env var for token

start a workspace using this repo as a base 

```sh
brev start https://github.com/brevdev/brev-cli
```
in this repo in `~/workspace`  run the commands from [new linux runner](https://github.com/brevdev/brev-cli/settings/actions/runners/new?arch=x64&os=linux)

```sh
mkdir actions-runner && cd actions-runner
curl -o actions-runner-linux-x64-2.294.0.tar.gz -L https://github.com/actions/runner/releases/download/v2.294.0/actions-runner-linux-x64-2.294.0.tar.gz
tar xzf ./actions-runner-linux-x64-2.294.0.tar.gz
./config.sh --url https://github.com/brevdev/brev-cli --token $TOKEN 
./run.sh
```
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
