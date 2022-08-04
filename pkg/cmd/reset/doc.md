## SYNOPSIS

```
    brev reset [ Workspace Name or ID... ]
```

## DESCRIPTION

reset a workspace will stop a workspace, then start a workspace, perserving
files in `/home/brev/workspace/`. This will have the effect of rerunning your
setupscript in a newley created workspace with no changes made to it, and
replacing your workspace with that.

## EXAMPLE

reset a workspace with the name `payments-frontend`

```
$ brev reset payments-frontend
Workspace payments-frontend is resetting.
Note: this can take a few seconds. Run 'brev ls' to check status

```

## SEE ALSO

    TODO
