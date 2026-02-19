# Instance Cleanup Workflow

Review and clean up unused GPU instances.

## Step 1: List All Instances

```bash
brev ls
```

Parse output to identify:
- Instance name
- Status (running/stopped)
- Instance type
- Creation time (if available)

## Step 2: Identify Candidates for Cleanup

Look for:
- Stopped instances (not in use)
- Old instances (name suggests temporary use)
- Test/dev instances

Present list to user:
```
Found X instances:

Running:
  - prod-training (H100, running) - Keep?
  - ml-experiment (A100, running) - Keep?

Stopped:
  - old-test (g5.xlarge, stopped) - Delete?
  - temp-dev (g4dn.xlarge, stopped) - Delete?
```

## Step 3: Confirm Deletions

Use AskUserQuestion with multiSelect:
```
question: "Which instances should we delete?"
header: "Cleanup"
multiSelect: true
options:
  - label: "<instance-1>"
    description: "<type>, <status>"
  - label: "<instance-2>"
    description: "<type>, <status>"
```

## Step 4: Delete Selected Instances

For each confirmed instance:
```bash
brev delete <instance-name>
```

Report success/failure for each.

## Step 5: Verify Cleanup

```bash
brev ls
```

Confirm deleted instances are gone.

## Safety Checks

**Before deleting, ALWAYS:**
- Confirm instance name with user
- Warn if instance is running
- Suggest copying important data first:
  ```bash
  brev copy <name>:/home/ubuntu/important/ ./backup/
  ```

**NEVER delete without explicit confirmation for each instance.**
