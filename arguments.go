package cli

import (
	"fmt"
	"strings"
	"unicode"
)

type signature struct {
	Command string
	Args    []argument
}

type argument struct {
	Name       string
	HasDefault bool
	Default    string
}

func parseSignature(sig string) (signature, error) {
	tokens := strings.Fields(sig)
	if len(tokens) == 0 {
		return signature{}, fmt.Errorf("signature cannot be empty")
	}

	argTokens := tokens
	parsed := signature{
		Args: make([]argument, 0, len(tokens)),
	}

	if !strings.HasPrefix(tokens[0], "{") {
		parsed.Command = tokens[0]
		argTokens = tokens[1:]

		if err := validateCommandName(parsed.Command); err != nil {
			return signature{}, err
		}
	}

	for _, token := range argTokens {
		arg, err := parseArgument(token)
		if err != nil {
			return signature{}, err
		}

		parsed.Args = append(parsed.Args, arg)
	}

	if err := validateSignature(parsed); err != nil {
		return signature{}, err
	}

	return parsed, nil
}

func parseArgument(token string) (argument, error) {
	if len(token) < 3 || token[0] != '{' || token[len(token)-1] != '}' {
		return argument{}, fmt.Errorf("invalid argument %q: arguments must be wrapped in braces", token)
	}

	raw := token[1 : len(token)-1]
	if raw == "" {
		return argument{}, fmt.Errorf("invalid argument %q: argument name cannot be empty", token)
	}

	arg := argument{}
	if strings.ContainsAny(raw, "?*") {
		return argument{}, fmt.Errorf("invalid argument %q: optional and repeated markers are inferred from the handler", token)
	}

	if strings.Contains(raw, "=") {
		name, defaultValue, _ := strings.Cut(raw, "=")
		arg.Name = name
		arg.HasDefault = true
		arg.Default = defaultValue
	} else {
		arg.Name = raw
	}

	if err := validateArgumentName(arg.Name); err != nil {
		return argument{}, fmt.Errorf("invalid argument %q: %w", token, err)
	}

	return arg, nil
}

func validateSignature(sig signature) error {
	seenNames := make(map[string]struct{}, len(sig.Args))

	for _, arg := range sig.Args {
		if _, exists := seenNames[arg.Name]; exists {
			return fmt.Errorf("duplicate argument name %q", arg.Name)
		}
		seenNames[arg.Name] = struct{}{}
	}

	return nil
}

func validateCommandName(name string) error {
	if name == "" {
		return fmt.Errorf("command name cannot be empty")
	}
	if strings.ContainsAny(name, "{}") {
		return fmt.Errorf("invalid command name %q", name)
	}

	return nil
}

func validateArgumentName(name string) error {
	if name == "" {
		return fmt.Errorf("argument name cannot be empty")
	}

	for i, r := range name {
		switch {
		case unicode.IsLetter(r), r == '_':
		case i > 0 && unicode.IsDigit(r):
		case i > 0 && r == '-':
		default:
			return fmt.Errorf("argument name %q contains invalid characters", name)
		}
	}

	return nil
}
