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

to generate a new file in `pkg/cmd/<command>` run:

```
make new-cmd name=<command>
```
in `pkg/cmd/<command>` you should see an output similar to:

```go
package envsetup

import (
	"github.com/spf13/cobra"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

type envsetupStore interface{}

func NewCMDenvsetup(t *terminal.Terminal, store envsetupStore) *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "envsetup",
		DisableFlagsInUseLine: true,
		Short:                 "TODO",
		Long:                  "TODO",
		Example:               "TODO",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := Runenvsetup(t, args, store)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}
	return cmd
}

func Runenvsetup(t *terminal.Terminal, args []string, store envsetupStore) error {
	return nil
}
```

after you have implemented `Runenvsetup`, add the command to `pkg/cmd/cmd.go`

```diff
diff --git a/pkg/cmd/cmd.go b/pkg/cmd/cmd.go
index a33540c..b03d5f2 100644
--- a/pkg/cmd/cmd.go
+++ b/pkg/cmd/cmd.go
@@ -23,6 +23,7 @@ import (
        "github.com/brevdev/brev-cli/pkg/cmd/portforward"
        "github.com/brevdev/brev-cli/pkg/cmd/profile"
        "github.com/brevdev/brev-cli/pkg/cmd/proxy"
+       "github.com/brevdev/brev-cli/pkg/cmd/envsetup"
        "github.com/brevdev/brev-cli/pkg/cmd/refresh"
        "github.com/brevdev/brev-cli/pkg/cmd/reset"
        "github.com/brevdev/brev-cli/pkg/cmd/runtasks"
@@ -243,6 +244,7 @@ func createCmdTree(cmd *cobra.Command, t *terminal.Terminal, loginCmdStore *stor
        cmd.AddCommand(healthcheck.NewCmdHealthcheck(t, noLoginCmdStore))

        cmd.AddCommand(setupworkspace.NewCmdSetupWorkspace(noLoginCmdStore))
+       cmd.AddCommand(envsetupNewCMDenvsetup.(t, loginCmdStore))
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

## debugging envsetup with delve

some of our commands need to be run as root and vscode's debugging tools don't work well with that. Debugging with delve is an easy way to do that.


### installation

```
go get github.com/go-delve/delve/cmd/dlv
go install github.com/go-delve/delve/cmd/dlv
```

### usage

   set a breakpoint where it is relevant with `runtime.Breakpoint()`

```go
func (lsc LinuxSystemdConfigurer) Install() error {
	_ = lsc.UnInstall() // best effort
	runtime.Breakpoint()
	err := lsc.WriteString(lsc.DestConfigFile, lsc.ValueConfigFile)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	//nolint //this is never defined by a user
	out, err := exec.Command("systemctl", "enable", lsc.ServiceName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("error running systemctl enable %s: %v, %s", lsc.DestConfigFile, err, out)
	}
	//nolint //this is never defined by a user
	out, err = exec.Command("systemctl", "start", lsc.ServiceName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("error running systemctl start %s: %v, %s", lsc.DestConfigFile, err, out)
	}
	return nil
}
```

   from the project root run

```sh
dlv debug github.com/brevdev/brev-cli -- envsetup --debugger
```


   the first prompt is before the program runs, and in this prompt hit `c` to continue to the first breakpoint

``` shell
$ dlv debug github.com/brevdev/brev-cli -- tasks configure --all --user $UID
Type 'help' for list of commands.
(dlv) c
> [hardcoded-breakpoint] github.com/brevdev/brev-cli/pkg/autostartconf.LinuxSystemdConfigurer.Install() ./pkg/autostartconf/linux.go:25 (hits total:0) (PC: 0x15946fc)
    20: }
    21:
    22: func (lsc LinuxSystemdConfigurer) Install() error {
    23:         _ = lsc.UnInstall() // best effort
    24:         runtime.Breakpoint()
=>  25:         err := lsc.WriteString(lsc.DestConfigFile, lsc.ValueConfigFile)
    26:         if err != nil {
    27:                 return breverrors.WrapAndTrace(err)
    28:         }
    29:         //nolint //this is never defined by a user
    30:         out, err := exec.Command("systemctl", "enable", lsc.ServiceName).CombinedOutput()
(dlv)
```

   view the value of a variable with `print`

```sh
(dlv) print lsc
github.com/brevdev/brev-cli/pkg/autostartconf.LinuxSystemdConfigurer {
        AutoStartStore: github.com/brevdev/brev-cli/pkg/autostartconf.AutoStartStore nil,
        ValueConfigFile: "\n[Install]\nWantedBy=multi-user.target\n\n[Unit]\nDescription=Brev S...+126 more",
        DestConfigFile: "/etc/systemd/system/brevvpnd.service",
        ServiceName: "brevvpnd",}
(dlv) print lsc.AutoStartStore
github.com/brevdev/brev-cli/pkg/autostartconf.AutoStartStore nil
(dlv)
```

   exit the debugger with `exit`

```sh
(dlv) exit
```

   make sure that you remove your breakpoints when finished



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

e2e tests are tests that spawn a docker container and runs brev setupworkspace
inside of it.

The can be run with the command

```
make e2e-test
```

they are also run with github actions when your commit contains the text `e2e-test`.

its convienent to run the e2e tests on github because you can run them in parallel,
and they take less time to run than running them sequentially.

### generate workflows for github actions

It takes forever to run these sequentially, so we use github actions to run them in parallel. I tried running them sequentially in github actions, but it timed out.
to generate the workflows for github actions, run:

```
make gen-e2e
```

### configure a runner fo e2e tests

start a workspace using this repo as a base

```sh
brev start https://github.com/brevdev/brev-cli -n bcli-runner-0
```

open a shell in your environment

```sh
brev shell bcli-runner-0
```

in this repo in `~/workspace`, run:

create `~/workspace/actions-runner` directory, and install actions-runner into
it

```sh
mkdir ~/workspace/actions-runner && cd ~/workspace/actions-runner
curl -o actions-runner-linux-x64-2.294.0.tar.gz -L https://github.com/actions/runner/releases/download/v2.294.0/actions-runner-linux-x64-2.294.0.tar.gz
tar xzf ./actions-runner-linux-x64-2.294.0.tar.gz
```

get the configure command and token from from [new linux runner](https://github.com/brevdev/brev-cli/settings/actions/runners/new?arch=x64&os=linux)

```
./config.sh --url https://github.com/brevdev/brev-cli --token  --unattended
```

create a systemd service to run the actions runner

switch to root

```sh
sudo su
```

```sh
cat <<EOF > /etc/systemd/system/actions-runner.service
[Unit]
Description=github actions runner for brev-cli
Requires=docker.service
After=docker.service

[Service]
ExecStart=/bin/zsh -l -c "cd /home/brev/workspace/actions-runner && ./run.sh"
Restart=always
RestartSec=10
User=brev

[Install]
WantedBy=multi-user.target
EOF
```

optionally switch back to brev user

```sh
su brev
```

start and enable the service

```sh
sudo systemctl start actions-runner.service
sudo systemctl enable actions-runner.service
```

view the logs to make sure it is working

```sh
sudo journalctl -f -xeu actions-runner.service
```

which should have an output similar to

```
Aug 05 18:09:57 w8s-ghub-runner-xwdm-brev-new-5ffb99758d-vdjdn bash[441429]: √ Connected to GitHub
Aug 05 18:09:58 w8s-ghub-runner-xwdm-brev-new-5ffb99758d-vdjdn bash[441429]: Current runner version: '2.294.0'
Aug 05 18:09:58 w8s-ghub-runner-xwdm-brev-new-5ffb99758d-vdjdn bash[441429]: 2022-08-05 18:09:58Z: Listening for Jobs
```

log into docker to avoid getting rate limited

```
docker login
```

viewing logs on a remote machine

1.  get the ssh host key from the remote machine

```sh
brev ls
```

which has an output similar to

```
λ brev ls
You have 3 workspaces in Org brev-new
 NAME                      STATUS   URL                                                                 ID
 ghub-runner               RUNNING  ghub-runner-xwdm-brev-new.wgt-us-west-2-test.brev.dev               sl9b6xwdm
 ghub-runner-1             RUNNING  ghub-runner-1-mtpe-brev-new.wgt-us-west-2-test.brev.dev             4rl8nmtpe
 ghub-runner-2             RUNNING  ghub-runner-2-lz9m-brev-new.wgt-us-west-2-test.brev.dev             4tzjilz9m

Connect to running workspace:
	brev open brev-cli	# brev open <NAME> -> open workspace in preferred editor
	brev shell brev-cli	# brev shell <NAME> -> ssh into workspace (shortcut)
	ssh brev-cli-p7gs	# ssh <SSH-NAME> -> ssh directly to workspace
Or ssh:
	ssh ghub-runner-xwdm
	ssh ghub-runner-1-mtpe
	ssh ghub-runner-2-lz9m
                                                                                                                                               Py base
~
λ
```

to get the logs from `ghub-runner`, you can use ssh to connect to the workspace,
run a command, and then send the output back to your local machine.

```sh
ssh ghub-runner-xwdm sudo journalctl -f  -xeu actions-runner.service
```

#### debugging notes

when editing the service file, run

```
sudo systemctl daemon-reload
sudo systemctl restart actions-runner.service
sudo journalctl -f -xeu actions-runner.service
```

to have it take effect

### remove queued jobs from github actions

sometimes, if a runner has not been allocated for a while, there will be a bunch
of queued jobs. To remove them, set your github token and run:

[create a personal access token](https://github.com/settings/tokens)

```
export GH_TOKEN=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
make remove-queued-jobs
```

## Known issues:

- sometimes github rate limits pulls.
- on first run, the runner will take a while pulling the workspace image, which
  causes the e2e tests to fail b/c they will timeout.

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

## Note for admins

default configuration is broken for admins, add this config to your `~/.brev/config.yaml`

```yaml
feature:
  not_admin: true
  service_mesh: false
```
