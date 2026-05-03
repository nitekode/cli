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
		if err := configureGroupFlags(group); err != nil {
			panic("cli: " + err.Error())
		}
	}

	for name, cmd := range app.commands {
		if name == "" || !strings.Contains(name, " ") {
			if err := configureCommandFlags(&cmd, app.flags); err != nil {
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

		defaultValue := field.Tag.Get("default")
		if defaultValue != "" {
			if err := validateFlagDefault(flagType, defaultValue); err != nil {
				return nil, fmt.Errorf("flag field %q: %w", field.Name, err)
			}
		}

		fields = append(fields, flagField{
			Name:        name,
			Description: field.Tag.Get("desc"),
			Default:     defaultValue,
			Index:       index,
			Type:        flagType,
			Bool:        flagType.Kind() == reflect.Bool,
		})
	}

	return fields, nil
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

func configureGroupFlags(group *group) error {
	group.flags = app.flags

	if group.localFlags != nil {
		if app.flags != nil && !hasEmbeddedFlagSet(group.localFlags.typ, app.flags.typ) {
			return fmt.Errorf("group %q flags must embed %s", group.name, app.flags.typ.Name())
		}
		group.flags = group.localFlags
	}

	for name, cmd := range group.commands {
		if err := configureCommandFlags(&cmd, group.flags); err != nil {
			return err
		}
		group.commands[name] = cmd
	}

	return nil
}

func configureCommandFlags(cmd *command, parent *flagSet) error {
	cmd.flags = parent

	if cmd.localFlags != nil {
		if parent != nil && !hasEmbeddedFlagSet(cmd.localFlags.typ, parent.typ) {
			return fmt.Errorf("command %q flags must embed %s", cmd.name, parent.typ.Name())
		}
		cmd.flags = cmd.localFlags
	}

	switch {
	case cmd.flags == nil && cmd.handlerFlagsType != nil:
		return fmt.Errorf("command %q handler declares flags but no flags are registered", cmd.name)
	case cmd.flags != nil && cmd.handlerFlagsType == nil:
		return fmt.Errorf("command %q handler must accept %s flags", cmd.name, cmd.flags.typ.Name())
	case cmd.flags != nil && cmd.handlerFlagsType != cmd.flags.typ:
		return fmt.Errorf("command %q handler must accept %s flags", cmd.name, cmd.flags.typ.Name())
	}

	return nil
}

func parseFlags(set *flagSet, args []string) (reflect.Value, []string, error) {
	if set == nil {
		return reflect.Value{}, args, nil
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

	flagByName := make(map[string]flagField, len(set.fields))
	for _, field := range set.fields {
		flagByName[field.Name] = field
	}

	positionals := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		token := args[i]
		if token == "--" {
			positionals = append(positionals, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(token, "--") {
			positionals = append(positionals, token)
			continue
		}

		name := strings.TrimPrefix(token, "--")
		rawValue := ""
		if strings.Contains(name, "=") {
			name, rawValue, _ = strings.Cut(name, "=")
		}

		field, found := flagByName[name]
		if !found {
			return reflect.Value{}, nil, fmt.Errorf("unknown option --%s", name)
		}

		if field.Bool {
			if rawValue == "" {
				rawValue = "true"
			}
		} else if rawValue == "" {
			if i+1 >= len(args) {
				return reflect.Value{}, nil, fmt.Errorf("missing value for option --%s", name)
			}
			i++
			rawValue = args[i]
		}

		target := value.FieldByIndex(field.Index)
		if err := setFlagValue(target, field, rawValue); err != nil {
			return reflect.Value{}, nil, fmt.Errorf("invalid value for option --%s: %w", name, err)
		}
	}

	return value, positionals, nil
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
