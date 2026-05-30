package cli

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"unicode"

	"github.com/nitekode/reflector"
)

type flagField struct {
	Name        string
	Short       string
	Description string
	Default     string
	Bool        bool
	Count       bool
}

type flagSet struct {
	typ    reflect.Type
	fields []flagField
	fill   func(map[string]string) (any, error)
}

func GlobalFlags[T any]() {
	set, err := compileFlagSet[T]()
	if err != nil {
		panic("cli: " + err.Error())
	}

	app.flags = set

	for _, group := range app.groups {
		if err := validateGroupFlags(group); err != nil {
			panic("cli: " + err.Error())
		}
		for name, cmd := range group.commands {
			if err := validateCommandFlags(&cmd); err != nil {
				panic("cli: " + err.Error())
			}
			group.commands[name] = cmd
		}
	}

	for name, cmd := range app.commands {
		if name == "" || !strings.Contains(name, " ") {
			if err := validateCommandFlags(&cmd); err != nil {
				panic("cli: " + err.Error())
			}
			app.commands[name] = cmd
		}
	}
}

func Flags[T any]() flagsOption {
	set, err := compileFlagSet[T]()
	if err != nil {
		panic("cli: " + err.Error())
	}

	return flagsOption{flags: set}
}

func compileFlagSet[T any]() (*flagSet, error) {
	var zero T
	typ := reflect.TypeFor[T]()
	if typ.Kind() != reflect.Struct {
		return nil, fmt.Errorf("flags must be a struct")
	}

	si, err := reflector.InspectStruct(zero)
	if err != nil {
		return nil, err
	}
	if _, err := reflector.NewStruct[T](reflector.WithDefaultTag("default")); err != nil {
		return nil, err
	}

	fields := make([]flagField, 0, len(si.Fields))
	for _, field := range si.Fields {
		flagType := field.Type
		switch flagType.Kind() {
		case reflect.Bool, reflect.String, reflect.Int:
		default:
			return nil, fmt.Errorf("flag field %q must be bool, string, or int", field.Name)
		}

		name, short, err := parseFlagTag(field.Tags["flag"])
		if err != nil {
			return nil, fmt.Errorf("flag field %q: %w", field.Name, err)
		}
		if err := validateFlagName(name); err != nil {
			return nil, fmt.Errorf("flag field %q: %w", field.Name, err)
		}
		if short != "" {
			if err := validateFlagShort(short); err != nil {
				return nil, fmt.Errorf("flag field %q: %w", field.Name, err)
			}
		}

		defaultValue := field.Tags["default"]

		count := field.Tags["count"] == "true"
		if count && flagType.Kind() != reflect.Int {
			return nil, fmt.Errorf("flag field %q: count flag must be int", field.Name)
		}

		fields = append(fields, flagField{
			Name:        name,
			Short:       short,
			Description: field.Tags["desc"],
			Default:     defaultValue,
			Bool:        flagType.Kind() == reflect.Bool,
			Count:       count,
		})
	}

	seen := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		if _, found := seen[field.Name]; found {
			return nil, fmt.Errorf("duplicate flag %q", field.Name)
		}
		seen[field.Name] = struct{}{}

		if field.Short == "" {
			continue
		}
		if _, found := seen[field.Short]; found {
			return nil, fmt.Errorf("duplicate flag %q", field.Short)
		}
		seen[field.Short] = struct{}{}
	}

	return &flagSet{
		typ:    typ,
		fields: fields,
		fill: func(input map[string]string) (any, error) {
			value, err := reflector.NewStruct[T](reflector.WithDefaultTag("default"))
			if err != nil {
				return nil, err
			}
			if err := reflector.FillFromMap(&value, input, reflector.WithNameTag("flag")); err != nil {
				return nil, err
			}
			return value, nil
		},
	}, nil
}

func parseFlagTag(raw string) (name string, short string, err error) {
	if raw == "" {
		return "", "", fmt.Errorf(`missing required tag "flag"`)
	}

	parts := strings.Split(raw, ",")
	if len(parts) == 0 || len(parts) > 2 {
		return "", "", fmt.Errorf(`invalid flag tag %q`, raw)
	}

	name = strings.TrimSpace(parts[0])
	if name == "" {
		return "", "", fmt.Errorf("flag tag must include a long name")
	}

	if len(parts) == 2 {
		short = strings.TrimSpace(parts[1])
		if short == "" {
			return "", "", fmt.Errorf("flag tag short name cannot be empty")
		}
	}

	return name, short, nil
}

func validateFlagShort(short string) error {
	if len(short) != 1 {
		return fmt.Errorf("short flag %q must be a single character", short)
	}
	r := rune(short[0])
	if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
		return fmt.Errorf("invalid short flag %q", short)
	}
	return nil
}

func validateFlagName(name string) error {
	if name == "" {
		return fmt.Errorf("flag name cannot be empty")
	}
	for i, r := range name {
		switch {
		case unicode.IsLetter(r):
		case unicode.IsDigit(r) && i > 0:
		case r == '-' && i > 0:
		default:
			return fmt.Errorf("invalid flag name %q", name)
		}
	}
	return nil
}

func hasEmbeddedFlagSet(typ reflect.Type, parent reflect.Type) bool {
	si, err := reflector.InspectStruct(reflect.Zero(typ).Interface())
	if err != nil {
		return false
	}

	return si.Embeds(parent)
}

func validateGroupFlags(group *group) error {
	if group.flags != nil && app.flags != nil && !hasEmbeddedFlagSet(group.flags.typ, app.flags.typ) {
		return fmt.Errorf("group %q flags must embed %s", group.name, app.flags.typ.Name())
	}

	return nil
}

func validateCommandFlags(cmd *command) error {
	parent := cmd.parentFlags()
	if cmd.flags != nil && parent != nil && !hasEmbeddedFlagSet(cmd.flags.typ, parent.typ) {
		return fmt.Errorf("command %q flags must embed %s", cmd.name, parent.typ.Name())
	}
	effective := cmd.effectiveFlags()
	switch {
	case effective == nil && cmd.handlerFlagsType != nil:
		return fmt.Errorf("command %q handler declares flags but no flags are registered", cmd.name)
	case effective != nil && cmd.handlerFlagsType != nil && cmd.handlerFlagsType != effective.typ:
		return fmt.Errorf("command %q handler must accept %s flags", cmd.name, effective.typ.Name())
	}

	return nil
}

func (group *group) effectiveFlags() *flagSet {
	if group == nil {
		return nil
	}
	if group.flags != nil {
		return group.flags
	}
	return app.flags
}

func (cmd command) parentFlags() *flagSet {
	if cmd.group != nil {
		return cmd.group.effectiveFlags()
	}
	return app.flags
}

func (cmd command) effectiveFlags() *flagSet {
	if cmd.flags != nil {
		return cmd.flags
	}
	return cmd.parentFlags()
}

func parseFlags(set *flagSet, expectedPositionals []commandArgument, args []string) (any, []string, error) {
	booleanFlags := make([]string, 0)
	countFlags := make([]string, 0)
	if set == nil {
		parsed, err := parseCommandLine(args, booleanFlags, nil)
		if err != nil {
			return nil, nil, err
		}
		if err := parsed.validate(nil, expectedPositionals); err != nil {
			return nil, nil, err
		}
		return nil, parsed.positionals, nil
	}

	for _, field := range set.fields {
		if field.Count {
			countFlags = append(countFlags, field.Name)
			if field.Short != "" {
				countFlags = append(countFlags, field.Short)
			}
			continue
		}
		if !field.Bool {
			continue
		}
		booleanFlags = append(booleanFlags, field.Name)
		if field.Short != "" {
			booleanFlags = append(booleanFlags, field.Short)
		}
	}

	parsed, err := parseCommandLine(args, booleanFlags, countFlags)
	if err != nil {
		return nil, nil, err
	}
	if err := parsed.validate(set, expectedPositionals); err != nil {
		return nil, nil, err
	}

	input := make(map[string]string, len(parsed.options)+len(parsed.flags))

	for _, field := range set.fields {
		if field.Count {
			if n := parsed.counts[field.Name]; n > 0 {
				input[field.Name] = strconv.Itoa(n)
			}
			continue
		}
		if field.Bool {
			rawValue, found := parsed.flags[field.Name]
			if !found {
				continue
			}
			input[field.Name] = strconv.FormatBool(rawValue)
			continue
		}

		values := parsed.options[field.Name]
		for _, rawValue := range values {
			input[field.Name] = rawValue
		}
	}

	value, err := set.fill(input)
	if err != nil {
		var fieldErr *reflector.FieldError
		if errors.As(err, &fieldErr) {
			return nil, nil, fmt.Errorf("invalid value for option --%s: %w", fieldErr.Field, fieldErr.Err)
		}
		return nil, nil, err
	}

	return value, parsed.positionals, nil
}
