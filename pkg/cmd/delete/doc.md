# Delete a Workspace by name or ID.

## SYNOPSIS

```
    brev delete [ Workspace Name or ID... ]
```

## DESCRIPTION

Deleting a workspace will permanently delete a workspace from your account.
This command will delete all content in the workspace and any volumes associated
with the workspace. This command is not reversable and can result in lost work.

## EXAMPLE

Delete a workspace with the name `payments-frontend`

```
$ brev delete payments-frontend
Deleting workspace payments-frontend. This can take a few minutes. Run 'brev ls' to check status
```

## SEE ALSO

	TODO
