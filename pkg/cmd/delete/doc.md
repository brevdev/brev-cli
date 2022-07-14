Delete a Workspace by name or ID.

## SYNOPSIS

```
    brev delete [ Workspace Name or ID... ]
```

## DESCRIPTION

Deleting a workspace will permanently delete a workspace from your account.
This command will delete all content in the workspace and any volumes associated
with the workspace. This command is not reversable and can result in lost work.

## EXAMPLE

### Delete a workspace

```
$ brev delete payments-frontend
Deleting workspace payments-frontend. This can take a few minutes. Run 'brev ls' to check status
```

#### Delete multiple workspaces

```
$ brev delete bar euler54 naive-pubsub jupyter
Deleting workspace bar. This can take a few minutes. Run 'brev ls' to check status
Deleting workspace euler54. This can take a few minutes. Run 'brev ls' to check status
Deleting workspace naive-pubsub. This can take a few minutes. Run 'brev ls' to check status
Deleting workspace jupyter. This can take a few minutes. Run 'brev ls' to check status

```

## SEE ALSO

	TODO
