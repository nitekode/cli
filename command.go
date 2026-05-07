package cli

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

type argumentKind uint8

const (
	requiredArgument argumentKind = iota
	optionalArgument
	defaultArgument
	repeatedArgument
)

type commandArgument struct {
	Description string
	Name        string
	Kind        argumentKind
	Default     string
}

type command struct {
	description      string
	name             string
	arguments        []commandArgument
	flags            *flagSet
	group            *group
	handlerType      reflect.Type
	handler          reflect.Value
	handlerFlagsType reflect.Type
	hidden           bool
	middleware       []MiddlewareFunc
}

var errorType = reflect.TypeFor[error]()
var stringType = reflect.TypeFor[string]()

func newCommand(sig string, description string, handler any, opts ...CommandOption) (command, error) {
	parsedSig, err := parseSignature(sig)
	if err != nil {
		return command{}, err
	}
	if strings.TrimSpace(description) == "" {
		return command{}, errors.New("command description cannot be empty")
	}

	handlerValue := reflect.ValueOf(handler)
	if !handlerValue.IsValid() {
		return command{}, errors.New("handler must be a function")
	}

	handlerType := handlerValue.Type()
	arguments, handlerFlagsType, err := compileCommandArguments(parsedSig, handlerType)
	if err != nil {
		return command{}, err
	}

	cmd := command{
		description:      description,
		name:             parsedSig.Command,
		arguments:        arguments,
		handlerType:      handlerType,
		handler:          handlerValue,
		handlerFlagsType: handlerFlagsType,
	}

	for _, opt := range opts {
		opt.applyCommand(&cmd)
	}
	if err := validateCommandOptions(cmd); err != nil {
		return command{}, err
	}

	return cmd, nil
}

func compileCommandArguments(sig signature, handlerType reflect.Type) ([]commandArgument, reflect.Type, error) {
	if handlerType == nil || handlerType.Kind() != reflect.Func {
		return nil, nil, errors.New("handler must be a function")
	}

	if handlerType.NumOut() != 1 || !handlerType.Out(0).Implements(errorType) {
		return nil, nil, errors.New("handler must return a single error")
	}

	argOffset := 0
	var handlerFlagsType reflect.Type
	if handlerType.NumIn() > 0 && handlerType.In(0).Kind() == reflect.Struct {
		handlerFlagsType = handlerType.In(0)
		argOffset = 1
	}

	if handlerType.NumIn()-argOffset != len(sig.Args) {
		return nil, nil, fmt.Errorf(
			"handler for %q expects %d parameters, signature defines %d arguments",
			sig.Command,
			handlerType.NumIn()-argOffset,
			len(sig.Args),
		)
	}

	arguments := make([]commandArgument, len(sig.Args))
	for i, sigArg := range sig.Args {
		paramIndex := i + argOffset
		arg, err := compileCommandArgument(sigArg, handlerType.In(paramIndex), handlerType.IsVariadic() && paramIndex == handlerType.NumIn()-1)
		if err != nil {
			return nil, nil, err
		}

		arguments[i] = arg
	}

	return arguments, handlerFlagsType, nil
}

func compileCommandArgument(sigArg argument, paramType reflect.Type, variadic bool) (commandArgument, error) {
	arg := commandArgument{
		Description: sigArg.Description,
		Name:        sigArg.Name,
		Default:     sigArg.Default,
	}

	switch {
	case variadic:
		if paramType.Kind() != reflect.Slice || paramType.Elem() != stringType {
			return commandArgument{}, fmt.Errorf("argument %q must map to a ...string parameter", sigArg.Name)
		}
		if sigArg.HasDefault {
			return commandArgument{}, fmt.Errorf("argument %q cannot have a default because it is repeated", sigArg.Name)
		}
		arg.Kind = repeatedArgument
	case paramType == stringType:
		if sigArg.HasDefault {
			arg.Kind = defaultArgument
		} else {
			arg.Kind = requiredArgument
		}
	case paramType.Kind() == reflect.Pointer && paramType.Elem() == stringType:
		if sigArg.HasDefault {
			return commandArgument{}, fmt.Errorf("argument %q cannot have a default because it maps to a *string parameter", sigArg.Name)
		}
		arg.Kind = optionalArgument
	default:
		return commandArgument{}, fmt.Errorf("argument %q must map to string, *string, or ...string", sigArg.Name)
	}

	return arg, nil
}

func (cmd command) invoke(providedArgs []string, middleware ...MiddlewareFunc) error {
	next := func() error {
		flagsValue, positionals, err := parseFlags(cmd.effectiveFlags(), cmd.arguments, providedArgs)
		if err != nil {
			return err
		}

		inputs, err := bindInputs(cmd.arguments, cmd.handlerType, cmd.handlerFlagsType, flagsValue, positionals)
		if err != nil {
			return err
		}

		var results []reflect.Value
		if cmd.handlerType.IsVariadic() {
			results = cmd.handler.CallSlice(inputs)
		} else {
			results = cmd.handler.Call(inputs)
		}

		if !results[0].IsNil() {
			return results[0].Interface().(error)
		}

		return nil
	}

	for i := len(middleware) - 1; i >= 0; i-- {
		currentMiddleware := middleware[i]
		current := next
		next = func() error {
			return currentMiddleware(current)
		}
	}

	return next()
}

func bindInputs(args []commandArgument, handlerType reflect.Type, handlerFlagsType reflect.Type, flagsValue reflect.Value, providedArgs []string) ([]reflect.Value, error) {
	inputs := make([]reflect.Value, 0, len(args)+1)
	argOffset := 0
	if handlerFlagsType != nil {
		inputs = append(inputs, flagsValue)
		argOffset = 1
	}
	providedIndex := 0

	for i, arg := range args {
		paramType := handlerType.In(i + argOffset)

		if arg.Kind == repeatedArgument {
			values := reflect.MakeSlice(paramType, len(providedArgs)-providedIndex, len(providedArgs)-providedIndex)
			for j := providedIndex; j < len(providedArgs); j++ {
				values.Index(j - providedIndex).SetString(providedArgs[j])
			}
			inputs = append(inputs, values)
			providedIndex = len(providedArgs)
			continue
		}

		value, nextIndex, err := bindSingleArgument(arg, paramType, providedArgs, providedIndex)
		if err != nil {
			return nil, err
		}

		inputs = append(inputs, value)
		providedIndex = nextIndex
	}

	if providedIndex < len(providedArgs) {
		return nil, fmt.Errorf("too many arguments: got %d, want %d", len(providedArgs), len(args))
	}

	return inputs, nil
}

func bindSingleArgument(arg commandArgument, paramType reflect.Type, providedArgs []string, start int) (reflect.Value, int, error) {
	if start < len(providedArgs) {
		raw := providedArgs[start]
		if arg.Kind == optionalArgument {
			value := reflect.New(paramType.Elem())
			value.Elem().SetString(raw)
			return value, start + 1, nil
		}

		return reflect.ValueOf(raw), start + 1, nil
	}

	switch arg.Kind {
	case defaultArgument:
		return reflect.ValueOf(arg.Default), start, nil
	case optionalArgument:
		return reflect.Zero(paramType), start, nil
	default:
		return reflect.Value{}, start, fmt.Errorf("missing required argument %q", arg.Name)
	}
}

func (cmd command) usage(executable string) string {
	parts := make([]string, 0, len(cmd.arguments)+2)
	parts = append(parts, executable)

	if cmd.name != "" {
		parts = append(parts, cmd.name)
	}

	for _, arg := range cmd.arguments {
		parts = append(parts, formatUsageArgument(arg))
	}

	return strings.Join(parts, " ")
}

func (cmd command) argumentNames() []string {
	names := make([]string, 0, len(cmd.arguments))
	for _, arg := range cmd.arguments {
		names = append(names, arg.Name)
	}

	return names
}

func validateCommandOptions(cmd command) error {
	for _, arg := range cmd.arguments {
		if arg.Description == "" {
			continue
		}
		if arg.Name == "" {
			return fmt.Errorf("argument description must refer to a named argument")
		}
	}

	return nil
}

func formatUsageArgument(arg commandArgument) string {
	switch arg.Kind {
	case requiredArgument:
		return "<" + arg.Name + ">"
	case optionalArgument, defaultArgument:
		return "[" + arg.Name + "]"
	case repeatedArgument:
		return "[" + arg.Name + "...]"
	default:
		return arg.Name
	}
}

func commandLeafName(name string) string {
	if i := strings.LastIndexByte(name, ' '); i >= 0 {
		return name[i+1:]
	}

	return name
}
