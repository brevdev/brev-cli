package launch

import (
	"context"
	"fmt"
	"strings"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

type vmLaunchArgs struct {
	terminal *terminal.Terminal
	build    *store.VMBuild
	env      []string
	options  localOptions
	deps     launchDeps
}

type confirmFunc func(label string) bool

func runVM(ctx context.Context, args vmLaunchArgs) error {
	script := args.build.LifeCycleScriptAttr
	if script == nil || strings.TrimSpace(script.Script) == "" {
		args.terminal.Vprint("This VM launchable has no startup script; there is nothing to run locally.")
		return nil
	}
	name := strings.TrimSpace(script.Name)
	if name == "" {
		name = "The launchable startup script"
	}
	args.terminal.Vprintf("Warning: %s will run directly on this machine and may modify local files, packages, and services.\n", name)
	if !args.options.approve && !args.deps.confirm("Run this startup script locally?") {
		args.terminal.Vprint("Local launch canceled.")
		return nil
	}
	shell, err := localShell(args.deps.runner)
	if err != nil {
		return err
	}
	spec := commandSpec{
		name:   shell,
		args:   []string{"-c", script.Script},
		env:    args.env,
		stdin:  args.options.stdin,
		stdout: args.options.stdout,
		stderr: args.options.stderr,
	}
	if err := args.deps.runner.Run(ctx, spec); err != nil {
		return fmt.Errorf("run launchable startup script locally: %w", err)
	}
	args.terminal.Vprint("Local startup script completed successfully.")
	return nil
}

func localShell(runner commandRunner) (string, error) {
	if shell, err := runner.LookPath("bash"); err == nil {
		return shell, nil
	}
	if shell, err := runner.LookPath("sh"); err == nil {
		return shell, nil
	}
	return "", breverrors.NewValidationError("local VM launchables require bash or sh")
}

func confirmStartupScript(label string) bool {
	return terminal.PromptSelectInput(terminal.PromptSelectContent{
		Label: label,
		Items: []string{"Yes, proceed", "No, cancel"},
	}) == "Yes, proceed"
}
