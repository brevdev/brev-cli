package launch

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
)

type parameterBindingArgs struct {
	parameters []store.Parameter
	values     map[string]string
	secrets    map[string]store.ManagedSecretReference
	resolver   managedSecretResolver
}

func parseParameterValues(values []string) (map[string]string, error) {
	parsed := make(map[string]string, len(values))
	for _, item := range values {
		name, value, ok := strings.Cut(item, "=")
		name = strings.TrimSpace(name)
		if !ok || name == "" {
			return nil, breverrors.NewValidationError(fmt.Sprintf("invalid --param %q: expected NAME=VALUE", item))
		}
		if _, exists := parsed[name]; exists {
			return nil, breverrors.NewValidationError(fmt.Sprintf("parameter %q was provided more than once", name))
		}
		parsed[name] = value
	}
	return parsed, nil
}

func parseParameterSecrets(values []string) (map[string]store.ManagedSecretReference, error) {
	parsed := make(map[string]store.ManagedSecretReference, len(values))
	for _, item := range values {
		name, selector, ok := strings.Cut(item, "=")
		name = strings.TrimSpace(name)
		selector = strings.TrimSpace(selector)
		if !ok || name == "" || selector == "" {
			return nil, breverrors.NewValidationError(fmt.Sprintf(
				"invalid --param-secret %q: expected NAME=SECRET_ID[:VERSION_ID]", item,
			))
		}
		if strings.Contains(selector, "@") {
			return nil, breverrors.NewValidationError(fmt.Sprintf(
				"invalid --param-secret %q: use ':' between SECRET_ID and VERSION_ID", item,
			))
		}
		if _, exists := parsed[name]; exists {
			return nil, breverrors.NewValidationError(fmt.Sprintf("secret parameter %q was provided more than once", name))
		}
		secretID, versionID, hasVersion := strings.Cut(selector, ":")
		secretID = strings.TrimSpace(secretID)
		versionID = strings.TrimSpace(versionID)
		if secretID == "" || (hasVersion && versionID == "") {
			return nil, breverrors.NewValidationError(fmt.Sprintf("invalid --param-secret %q", item))
		}
		parsed[name] = store.ManagedSecretReference{SecretID: secretID, VersionID: versionID}
	}
	return parsed, nil
}

func resolveParameterBindings(
	ctx context.Context,
	args parameterBindingArgs,
) ([]store.ParameterBinding, error) {
	problems := validateParameterSelections(args.parameters, args.values, args.secrets)
	bindings, bindingProblems := buildParameterBindings(args.parameters, args.values, args.secrets)
	problems = append(problems, bindingProblems...)
	if len(problems) > 0 {
		sort.Strings(problems)
		return nil, breverrors.NewValidationError("invalid launchable parameters:\n  - " + strings.Join(problems, "\n  - "))
	}
	if err := resolveSecretVersions(ctx, bindings, args.resolver); err != nil {
		return nil, err
	}
	return bindings, nil
}

func validateParameterSelections(
	parameters []store.Parameter,
	values map[string]string,
	secrets map[string]store.ManagedSecretReference,
) []string {
	defined := make(map[string]store.Parameter, len(parameters))
	for _, parameter := range parameters {
		defined[parameter.Name] = parameter
	}

	var problems []string
	for name := range values {
		if _, ok := defined[name]; !ok {
			problems = append(problems, fmt.Sprintf("unknown parameter %q", name))
		}
	}
	for name := range secrets {
		parameter, ok := defined[name]
		if !ok {
			problems = append(problems, fmt.Sprintf("unknown secret parameter %q", name))
			continue
		}
		if _, hasValue := values[name]; hasValue {
			problems = append(problems, fmt.Sprintf("parameter %q cannot use both --param and --param-secret", name))
		}
		if parameter.Choice != nil {
			problems = append(problems, fmt.Sprintf("choice parameter %q cannot be bound to a secret", name))
		}
	}
	return problems
}

func buildParameterBindings(
	parameters []store.Parameter,
	values map[string]string,
	secrets map[string]store.ManagedSecretReference,
) ([]store.ParameterBinding, []string) {
	var problems []string
	bindings := make([]store.ParameterBinding, 0, len(parameters))
	for _, parameter := range parameters {
		if ref, ok := secrets[parameter.Name]; ok {
			bindings = append(bindings, store.ParameterBinding{Name: parameter.Name, ManagedSecret: &ref})
			continue
		}
		value := values[parameter.Name]
		if value == "" {
			value = parameterDefault(parameter)
		}
		if parameter.Required && value == "" {
			problems = append(problems, fmt.Sprintf("missing required parameter %q", parameter.Name))
			continue
		}
		if parameter.Choice != nil && value != "" && !slices.Contains(parameter.Choice.Choices, value) {
			problems = append(problems, fmt.Sprintf(
				"invalid value %q for %q (allowed: %s)", value, parameter.Name, strings.Join(parameter.Choice.Choices, ", "),
			))
			continue
		}
		if value != "" {
			bindings = append(bindings, store.ParameterBinding{Name: parameter.Name, Value: value})
		}
	}
	return bindings, problems
}

func resolveSecretVersions(ctx context.Context, bindings []store.ParameterBinding, resolver managedSecretResolver) error {
	for i := range bindings {
		ref := bindings[i].ManagedSecret
		if ref == nil || ref.VersionID != "" {
			continue
		}
		if resolver == nil {
			return fmt.Errorf("managed-secret resolver is not configured")
		}
		versionID, err := resolver.LatestVersion(ctx, ref.SecretID)
		if err != nil {
			return fmt.Errorf("resolve latest version for managed secret %q: %w", ref.SecretID, err)
		}
		ref.VersionID = versionID
	}
	return nil
}

func parameterDefault(parameter store.Parameter) string {
	if parameter.Text != nil {
		return parameter.Text.DefaultValue
	}
	if parameter.Choice != nil {
		return parameter.Choice.DefaultValue
	}
	return ""
}

func parameterDisplayLines(parameters []store.Parameter) []string {
	if len(parameters) == 0 {
		return nil
	}
	ordered := append([]store.Parameter(nil), parameters...)
	sort.Slice(ordered, func(i int, j int) bool {
		if ordered[i].Required != ordered[j].Required {
			return ordered[i].Required
		}
		return ordered[i].Name < ordered[j].Name
	})
	lines := []string{"Parameters:"}
	for _, parameter := range ordered {
		requirement := "optional"
		if parameter.Required {
			requirement = "required"
		}
		details := requirement
		if value := parameterDefault(parameter); value != "" {
			details += ", default: " + value
		}
		if parameter.Choice != nil {
			details += ", choices: " + strings.Join(parameter.Choice.Choices, ", ")
		}
		line := fmt.Sprintf("  %s\t(%s)", parameter.Name, details)
		if description := strings.TrimSpace(parameter.Description); description != "" {
			line += "\t" + description
		}
		lines = append(lines, line)
	}
	return lines
}
