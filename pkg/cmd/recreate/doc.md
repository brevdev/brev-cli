# Re Create Workspace by name or ID.

## SYNOPSIS

```
    brev recreate [ Workspace Name or ID... ]
```

## DESCRIPTION

recreate a workspace is equivalent to running the following commands:

```
brev delete payments-fronted
brev start payments-frontend
```

This command has the effect of updating the base image of a workspace to the
latest. If your workspace has a git remote source, the workspace will start
with a fresh copy of the remote source and run the workspace setupscript.

## EXAMPLE

recreate a workspace with the name `naive-pubsub`

```
$ brev recreate naive-pubsub
recreating 🤙 This can take a couple of minutes.

Deleting instance - naive-pubsub.
Instance is starting. This can take up to 2 minutes the first time.
name naive-pubsub
template v7nd45zsc Admin
resource class 4x16
workspace group brev-test-brevtenant-cluster
You can safely ctrl+c to exit
⢿  workspace is deploying
Your workspace is ready!

SSH into your machine:
        ssh naive-pubsub-uq0x
```

## SEE ALSO

    TODO
