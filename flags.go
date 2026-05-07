package cli

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"unicode"
)

type flagField struct {
	Name        string
	Short       string
	Description string
	Default     string
	Index       []int
	Type        reflect.Type
	Bool        bool
}

type flagSet struct {
	typ    reflect.Type
	fields []flagField
}

func GlobalFlags(flags any) {
	set, err := compileFlagSet(flags)
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

func Flags(flags any) flagsOption {
	set, err := compileFlagSet(flags)
	if err != nil {
		panic("cli: " + err.Error())
	}

	return flagsOption{flags: set}
}

func compileFlagSet(flags any) (*flagSet, error) {
	typ := reflect.TypeOf(flags)
	if typ == nil {
		return nil, fmt.Errorf("flags must be a struct")
	}
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		return nil, fmt.Errorf("flags must be a struct")
	}

	fields, err := collectFlagFields(typ, nil)
	if err != nil {
		return nil, err
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

	return &flagSet{typ: typ, fields: fields}, nil
}

func collectFlagFields(typ reflect.Type, path []int) ([]flagField, error) {
	fields := make([]flagField, 0, typ.NumField())

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		index := append(append([]int(nil), path...), i)

		if field.Anonymous {
			embeddedType := field.Type
			if embeddedType.Kind() == reflect.Pointer {
				embeddedType = embeddedType.Elem()
			}
			if embeddedType.Kind() != reflect.Struct {
				return nil, fmt.Errorf("embedded field %q must be a struct", field.Name)
			}

			embeddedFields, err := collectFlagFields(embeddedType, index)
			if err != nil {
				return nil, err
			}
			fields = append(fields, embeddedFields...)
			continue
		}

		if field.PkgPath != "" {
			continue
		}

		flagType := field.Type
		switch flagType.Kind() {
		case reflect.Bool, reflect.String, reflect.Int:
		default:
			return nil, fmt.Errorf("flag field %q must be bool, string, or int", field.Name)
		}

		name := field.Tag.Get("cli")
		if name == "" {
			name = fieldNameToFlag(field.Name)
		}
		if err := validateFlagName(name); err != nil {
			return nil, fmt.Errorf("flag field %q: %w", field.Name, err)
		}
		short := field.Tag.Get("short")
		if short != "" {
			if err := validateFlagShort(short); err != nil {
				return nil, fmt.Errorf("flag field %q: %w", field.Name, err)
			}
		}

		defaultValue := field.Tag.Get("default")
		if defaultValue != "" {
			if err := validateFlagDefault(flagType, defaultValue); err != nil {
				return nil, fmt.Errorf("flag field %q: %w", field.Name, err)
			}
		}

		fields = append(fields, flagField{
			Name:        name,
			Short:       short,
			Description: field.Tag.Get("desc"),
			Default:     defaultValue,
			Index:       index,
			Type:        flagType,
			Bool:        flagType.Kind() == reflect.Bool,
		})
	}

	return fields, nil
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

func fieldNameToFlag(name string) string {
	var b strings.Builder
	for i, r := range name {
		if unicode.IsUpper(r) {
			if i > 0 {
				b.WriteByte('-')
			}
			b.WriteRune(unicode.ToLower(r))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
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

func validateFlagDefault(typ reflect.Type, value string) error {
	switch typ.Kind() {
	case reflect.Bool:
		_, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid bool default %q", value)
		}
	case reflect.Int:
		_, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid int default %q", value)
		}
	case reflect.String:
	default:
		return fmt.Errorf("unsupported flag type %s", typ)
	}

	return nil
}

func hasEmbeddedFlagSet(typ reflect.Type, parent reflect.Type) bool {
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if !field.Anonymous {
			continue
		}

		fieldType := field.Type
		if fieldType.Kind() == reflect.Pointer {
			fieldType = fieldType.Elem()
		}
		if fieldType == parent {
			return true
		}
	}

	return false
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

func parseFlags(set *flagSet, expectedPositionals []commandArgument, args []string) (reflect.Value, []string, error) {
	booleanFlags := make([]string, 0)
	if set == nil {
		parsed, err := parseCommandLine(args, booleanFlags)
		if err != nil {
			return reflect.Value{}, nil, err
		}
		if err := parsed.validate(nil, expectedPositionals); err != nil {
			return reflect.Value{}, nil, err
		}
		return reflect.Value{}, parsed.positionals, nil
	}

	for _, field := range set.fields {
		if !field.Bool {
			continue
		}
		booleanFlags = append(booleanFlags, field.Name)
		if field.Short != "" {
			booleanFlags = append(booleanFlags, field.Short)
		}
	}

	parsed, err := parseCommandLine(args, booleanFlags)
	if err != nil {
		return reflect.Value{}, nil, err
	}
	if err := parsed.validate(set, expectedPositionals); err != nil {
		return reflect.Value{}, nil, err
	}

	value := reflect.New(set.typ).Elem()
	for _, field := range set.fields {
		if field.Default == "" {
			continue
		}
		target := value.FieldByIndex(field.Index)
		if err := setFlagValue(target, field, field.Default); err != nil {
			return reflect.Value{}, nil, err
		}
	}

	for _, field := range set.fields {
		if field.Bool {
			rawValue, found := parsed.flags[field.Name]
			if !found {
				continue
			}
			target := value.FieldByIndex(field.Index)
			target.SetBool(rawValue)
			continue
		}

		values := parsed.options[field.Name]
		for _, rawValue := range values {
			target := value.FieldByIndex(field.Index)
			if err := setFlagValue(target, field, rawValue); err != nil {
				return reflect.Value{}, nil, fmt.Errorf("invalid value for option --%s: %w", field.Name, err)
			}
		}
	}

	return value, parsed.positionals, nil
}

func setFlagValue(target reflect.Value, field flagField, raw string) error {
	switch field.Type.Kind() {
	case reflect.Bool:
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return err
		}
		target.SetBool(value)
	case reflect.Int:
		value, err := strconv.Atoi(raw)
		if err != nil {
			return err
		}
		target.SetInt(int64(value))
	case reflect.String:
		target.SetString(raw)
	default:
		return fmt.Errorf("unsupported flag type %s", field.Type)
	}

	return nil
}
