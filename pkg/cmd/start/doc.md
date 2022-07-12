# start creates, starts, and joins you to workspaces

## SYNOPSIS

```

brev start {<workspace name or id or git url >... | -e} {-n | --name} {-c | --class}
    { -s | --setup-script} {-r | --setup-repo}] {-p | --setup-path }
    { -o | --org}

```

## DESCRIPTION

The behavior of brev start is dependent on the context that you are in. The most
basic invocation of this command is


## EXAMPLES

### create an empty workspace

```
$ brev start -e -n foo
```

which has an output similar too:

```
name foo
template 4nbb4lg2s ubuntu
resource class 2x8
workspace group brev-test-brevtenant-cluster
workspace is starting. this can take up to 2 minutes the first time.

you can safely ctrl+c to exit
⣽  workspace is deploying
your workspace is ready!

connect to the workspace:
	brev open foo	# brev open <name> -> open workspace in preferred editor
	brev shell foo	# brev shell <name> -> ssh into workspace (shortcut)
	ssh foo-8j4u	# ssh <ssh-name> -> ssh directly to workspace

```

or

```
$ brev start --empty --name foo
```

which has an output similar too:

```
name foo
template 4nbb4lg2s ubuntu
resource class 2x8
workspace group brev-test-brevtenant-cluster
workspace is starting. this can take up to 2 minutes the first time.

you can safely ctrl+c to exit
⣽  workspace is deploying
your workspace is ready!

connect to the workspace:
	brev open foo	# brev open <name> -> open workspace in preferred editor
	brev shell foo	# brev shell <name> -> ssh into workspace (shortcut)
	ssh foo-8j4u	# ssh <ssh-name> -> ssh directly to workspace

```

view your workspace with `brev ls`


### create a workspace, and do not block shell until workspace is created

use the `-d` or `--detached` flag to create a workspace and immediately exit
rather than wait for workspace to be successfully created before exiting.

```
$ brev start -d -e -n bar
```

which has an output similar too:

```
name bar
template 4nbb4lg2s ubuntu
resource class 2x8
workspace group brev-test-brevtenant-cluster
Workspace is starting. This can take up to 2 minutes the first time.
```

### Create a workspace from a file path

if in your current directory has a directory in it called `merge-json`, you can
create a workspace using the contents of that directory using
`brev start merge-json`

```
$ ls
merge-json
```

```
$ brev start merge-json

```

which has an output similar too:

```
Workspace is starting. This can take up to 2 minutes the first time.

name merge-json
template 4nbb4lg2s ubuntu
resource class 2x8
workspace group brev-test-brevtenant-cluster
You can safely ctrl+c to exit
⡿  workspace is deploying
Your workspace is ready!

Connect to the workspace:
	brev open merge-json	# brev open <NAME> -> open workspace in preferred editor
	brev shell merge-json	# brev shell <NAME> -> ssh into workspace (shortcut)
	ssh merge-json-wd6q	# ssh <SSH-NAME> -> ssh directly to workspace
```

### Create a workspace from a git repository


```
$ brev start https://github.com/brevdev/react-starter-app
```

which has an output similar too:

```
Workspace is starting. This can take up to 2 minutes the first time.

name react-starter-app
template 4nbb4lg2s ubuntu
resource class 2x8
workspace group brev-test-brevtenant-cluster
You can safely ctrl+c to exit
⣾  workspace is deploying
Your workspace is ready!

Connect to the workspace:
	brev open react-starter-app	# brev open <NAME> -> open workspace in preferred editor
	brev shell react-starter-app	# brev shell <NAME> -> ssh into workspace (shortcut)
	ssh react-starter-app-8v8p	# ssh <SSH-NAME> -> ssh directly to workspace

```

### Join a workspace in your orginization

view your orgs workspaces with `brev ls --all`. Workspaces in your org that you
have not joined appear at the bottom of the output.

```
$ brev ls --all
```

which has an output similar too:

```
You have 1 workspace in Org brev.dev
 NAME                             STATUS    URL                                                                       ID
 brev-cli                         RUNNING   brev-cli-p09m-brevdev.wgt-us-west-2-test.brev.dev                         x1yxqp09m

Connect to running workspace:
	brev open brev-cli	# brev open <NAME> -> open workspace in preferred editor
	brev shell brev-cli	# brev shell <NAME> -> ssh into workspace (shortcut)
	ssh brev-cli-p09m	# ssh <SSH-NAME> -> ssh directly to workspace

7 other projects in Org brev.dev
 NAME                        MEMBERS
 new-docs                          1
 brev-landing-page                 2
 todo-app                          1
 vagrant-guide                     1
 mern-template                     1
 solidity-nextjs-starter           1
 akka-http-quickstart-scala        1

Join a project:
	brev start new-docs

```

join the project new-docs

```
$ brev start new-docs
```

which has an output similar too:

```
Name flag omitted, using auto generated name: new-docs
Workspace is starting. This can take up to 2 minutes the first time.

name new-docs
template 4nbb4lg2s ubuntu
resource class 2x8
workspace group brev-test-brevtenant-cluster
You can safely ctrl+c to exit
⣟  workspace is deploying Connect to the workspace:
	brev open new-docs	# brev open <NAME> -> open workspace in preferred editor
	brev shell new-docs	# brev shell <NAME> -> ssh into workspace (shortcut)
	ssh new-docs-pek9	# ssh <SSH-NAME> -> ssh directly to workspace
```

### Start a stopped workspace

If you have already joined a workspace and have stopped it with `brev stop`,
you can start it again with `brev start`

view your current workspaces with `brev ls`

```
$ brev ls
```

which has an output similar too:

```
You have 1 workspace in Org brev.dev
 NAME                             STATUS     URL                                                                       ID
 linear-client                    STOPPED    linear-client-yw1a-brevdev.wgt-us-west-2-test.brev.dev                    gov5jyw1a

Connect to running workspace:
	brev open linear-client	# brev open <NAME> -> open workspace in preferred editor
	brev shell linear-client	# brev shell <NAME> -> ssh into workspace (shortcut)
	ssh linear-client-yw1a	# ssh <SSH-NAME> -> ssh directly to workspace

```
join the workspace
```
$ brev start linear-client
```

which has an output similar too:

```
Workspace linear-client is starting.
Note: this can take about a minute. Run 'brev ls' to check status

You can safely ctrl+c to exit
```
