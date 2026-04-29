package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

type usageError struct {
	body string
}

func (e usageError) Error() string {
	return e.body
}

var app = struct {
	name        string
	description string

	version string
	commit  string
	builtAt string

	in  io.Reader
	out io.Writer
	err io.Writer

	commands   map[string]command
	groups     map[string]*group
	middleware []MiddlewareFunc
}{
	name: filepath.Base(os.Args[0]),

	version: "dev",
	commit:  "unknown",
	builtAt: "unknown",

	in:  os.Stdin,
	out: os.Stdout,
	err: os.Stderr,

	commands: make(map[string]command),
	groups:   make(map[string]*group),
}

type GroupAdder interface {
	Command(sig string, description string, handler any, opts ...CommandOption)
}

func Name(name string) {
	app.name = name
}

func Description(description string) {
	app.description = description
}

func Version(version string) {
	app.version = version
}

func Build(version string, commit string, builtAt string) {
	app.version = version
	app.commit = commit
	app.builtAt = builtAt
}

func SetIn(in io.Reader) io.Reader {
	previous := app.in
	app.in = in
	return previous
}

func SetOut(out io.Writer) io.Writer {
	previous := app.out
	app.out = out
	return previous
}

func SetErr(err io.Writer) io.Writer {
	previous := app.err
	app.err = err
	return previous
}

func In() io.Reader {
	return app.in
}

func Out() io.Writer {
	return app.out
}

func Err() io.Writer {
	return app.err
}

func Command(sig string, description string, handler any, opts ...CommandOption) {
	cmd, err := newCommand(sig, description, handler, opts...)
	if err != nil {
		panic("cli: " + err.Error())
	}

	if _, exists := app.groups[cmd.name]; cmd.name != "" && exists {
		panic("cli: command name conflicts with existing group " + strconv.Quote(cmd.name))
	}

	app.commands[cmd.name] = cmd
}

func Group(name string, description string, register func(GroupAdder), opts ...GroupOption) {
	if err := validateGroupName(name); err != nil {
		panic("cli: " + err.Error())
	}
	if strings.TrimSpace(description) == "" {
		panic("cli: group description cannot be empty")
	}
	if isBuiltInCommandName(name) {
		panic("cli: group name conflicts with built-in command " + strconv.Quote(name))
	}
	if _, exists := app.commands[name]; exists {
		panic("cli: group name conflicts with existing command " + strconv.Quote(name))
	}
	if _, exists := app.groups[name]; exists {
		panic("cli: duplicate group " + strconv.Quote(name))
	}

	g := &group{
		description: description,
		name:        name,
		commands:    make(map[string]command),
	}
	for _, opt := range opts {
		opt.applyGroup(g)
	}
	app.groups[name] = g
	register(groupAdder{group: g})
}

func Run() {
	if len(os.Args) <= 1 && app.commands[""].handlerType == nil {
		printUsageAndExit()
	}

	if err := RunWith(os.Args); err != nil {
		executable := filepath.Base(os.Args[0])
		var usage usageError
		if errors.As(err, &usage) {
			fmt.Fprint(app.err, usage.body)
			os.Exit(2)
		}

		fmt.Fprintf(app.err, "%s: %v\n", executable, err)
		os.Exit(2)
	}
}

func RunWith(args []string) error {
	if len(args) <= 1 {
		if cmd, found := app.commands[""]; found {
			return cmd.invoke(nil, append([]MiddlewareFunc(nil), app.middleware...)...)
		}

		return errors.New("no command provided")
	}

	commandName := args[1]
	cmd, found := app.commands[commandName]
	if found {
		return cmd.invoke(args[2:], append(append([]MiddlewareFunc(nil), app.middleware...), cmd.middleware...)...)
	}
	if commandName == "help" {
		return runHelp(args)
	}
	if commandName == "version" {
		printVersion()
		return nil
	}

	if group, found := app.groups[commandName]; found {
		if len(args) <= 2 {
			return usageError{body: groupHelp(filepath.Base(args[0]), group)}
		}

		groupCommandName := args[2]
		groupCommand, found := group.commands[groupCommandName]
		if !found {
			return usageError{body: groupHelp(filepath.Base(args[0]), group)}
		}

		middleware := append([]MiddlewareFunc(nil), app.middleware...)
		middleware = append(middleware, group.middleware...)
		middleware = append(middleware, groupCommand.middleware...)
		return groupCommand.invoke(args[3:], middleware...)
	}

	root, hasRoot := app.commands[""]
	if !hasRoot {
		return fmt.Errorf("unknown command %q", commandName)
	}

	return root.invoke(args[1:], append(append([]MiddlewareFunc(nil), app.middleware...), root.middleware...)...)
}

func printUsageAndExit() {
	fmt.Fprint(app.out, globalHelp(filepath.Base(os.Args[0])))
	os.Exit(0)
}

func globalHelp(executable string) string {
	var b strings.Builder

	writeHelpHeader(&b)
	b.WriteString("Usage:\n")

	if _, hasRoot := app.commands[""]; hasRoot {
		fmt.Fprintf(&b, "  %s [arguments]\n", executable)
	}

	if hasNamedCommands() || len(app.groups) > 0 {
		fmt.Fprintf(&b, "  %s {command} [arguments]\n", executable)
	}

	if hasNamedCommands() || len(app.groups) > 0 {
		b.WriteString("\nCommands:\n")
		column := globalCommandDescriptionColumn()
		writeCommandSummaries(&b, "  ", commandNames(), commandDescription, column)
		for _, groupName := range groupNames() {
			writeGroupSummary(&b, groupName, column)
			groupCommands := groupCommandNames(groupName)
			writeCommandSummaries(&b, "    ", groupCommands, func(commandName string) string {
				return groupCommandDescription(groupName, commandName)
			}, func(commandName string) string {
				return groupName + " " + commandName
			}, column)
		}
	}

	return b.String()
}

func commandHelp(executable string, cmd command) string {
	var b strings.Builder

	if cmd.description != "" {
		b.WriteString(cmd.description)
		b.WriteString("\n\n")
	}
	b.WriteString("Usage:\n")
	fmt.Fprintf(&b, "  %s\n", cmd.usage(executable))

	if len(cmd.arguments) > 0 {
		b.WriteString("\nArguments:\n")
		for _, arg := range cmd.arguments {
			suffix := ""
			if arg.Kind == defaultArgument {
				suffix = fmt.Sprintf(" (default=%s)", arg.Default)
			}
			if arg.Description == "" {
				fmt.Fprintf(&b, "  %s%s\n", arg.Name, suffix)
				continue
			}
			fmt.Fprintf(&b, "  %s  %s%s\n", arg.Name, arg.Description, suffix)
		}
	}

	return b.String()
}

func printVersion() {
	fmt.Fprintln(app.out, versionString())
}

func versionString() string {
	var b strings.Builder

	b.WriteString(app.version)

	if app.builtAt != "" && app.builtAt != "unknown" {
		b.WriteString(" (")
		b.WriteString(app.builtAt)
		b.WriteString(")")
	}

	if app.commit != "" && app.commit != "unknown" {
		b.WriteString(" [")
		b.WriteString(app.commit)
		b.WriteString("]")
	}

	return b.String()
}

func groupHelp(executable string, group *group) string {
	var b strings.Builder

	if group.description != "" {
		b.WriteString(group.description)
		b.WriteString("\n\n")
	}
	b.WriteString("Usage:\n")
	fmt.Fprintf(&b, "  %s %s {command} [arguments]\n", executable, group.name)

	if len(group.commands) > 0 {
		b.WriteString("\nCommands:\n")
		writeCommandSummaries(&b, "  ", groupCommandNames(group.name), func(commandName string) string {
			return groupCommandDescription(group.name, commandName)
		})
	}

	return b.String()
}

func writeHelpHeader(b *strings.Builder) {
	if app.description != "" {
		b.WriteString(app.description)
		b.WriteString("\n")
	}

	b.WriteString("\n")
}

func runHelp(args []string) error {
	executable := filepath.Base(args[0])
	switch len(args) {
	case 2:
		fmt.Fprint(app.out, globalHelp(executable))
		return nil
	case 3:
		name := args[2]
		if cmd, found := app.commands[name]; found {
			fmt.Fprint(app.out, commandHelp(executable, cmd))
			return nil
		}
		if group, found := app.groups[name]; found {
			fmt.Fprint(app.out, groupHelp(executable, group))
			return nil
		}
		if name == "help" {
			fmt.Fprintf(app.out, "Usage:\n  %s help [command]\n", executable)
			return nil
		}
		if name == "version" {
			fmt.Fprintf(app.out, "Usage:\n  %s version\n", executable)
			return nil
		}
	default:
		groupName := args[2]
		commandName := args[3]
		if group, found := app.groups[groupName]; found {
			if cmd, found := group.commands[commandName]; found {
				fmt.Fprint(app.out, commandHelp(executable, cmd))
				return nil
			}
		}
	}

	return fmt.Errorf("unknown help topic %q", strings.Join(args[2:], " "))
}

func hasNamedCommands() bool {
	if hasAutoBuiltInCommands() {
		return true
	}

	for name, cmd := range app.commands {
		if name != "" && !cmd.hidden {
			return true
		}
	}

	return false
}

func commandNames() []string {
	names := make([]string, 0, len(app.commands))
	if hasAutoBuiltInCommands() {
		if _, found := app.commands["help"]; !found {
			names = append(names, "help")
		}
		if _, found := app.commands["version"]; !found {
			names = append(names, "version")
		}
	}

	for name := range app.commands {
		if name == "" || app.commands[name].hidden {
			continue
		}

		names = append(names, name)
	}

	slices.Sort(names)
	return names
}

func hasAutoBuiltInCommands() bool {
	return hasAutoBuiltInCommand("help") || hasAutoBuiltInCommand("version")
}

func hasAutoBuiltInCommand(name string) bool {
	_, found := app.commands[name]
	return !found
}

func isBuiltInCommandName(name string) bool {
	return name == "help" || name == "version"
}

func groupNames() []string {
	names := make([]string, 0, len(app.groups))
	for name := range app.groups {
		if app.groups[name].hidden {
			continue
		}
		names = append(names, name)
	}

	slices.Sort(names)
	return names
}

func groupCommandNames(groupName string) []string {
	group, found := app.groups[groupName]
	if !found {
		return nil
	}

	names := make([]string, 0, len(group.commands))
	for name := range group.commands {
		if group.commands[name].hidden {
			continue
		}
		names = append(names, name)
	}

	slices.Sort(names)
	return names
}

func writeCommandSummaries(
	b *strings.Builder,
	indent string,
	names []string,
	description func(string) string,
	options ...any,
) {
	display := func(name string) string { return name }
	column := 0
	for _, option := range options {
		switch option := option.(type) {
		case func(string) string:
			display = option
		case int:
			column = option
		}
	}

	if column == 0 {
		for _, name := range names {
			if n := len(indent) + len(display(name)) + 2; n > column {
				column = n
			}
		}
	}

	for _, name := range names {
		label := display(name)
		desc := description(name)
		if desc == "" {
			fmt.Fprintf(b, "%s%s\n", indent, label)
			continue
		}
		padding := column - len(indent) - len(label)
		if padding < 2 {
			padding = 2
		}
		fmt.Fprintf(b, "%s%s%s%s\n", indent, label, strings.Repeat(" ", padding), desc)
	}
}

func globalCommandDescriptionColumn() int {
	column := 0

	for _, name := range commandNames() {
		if n := len("  ") + len(name) + 2; n > column {
			column = n
		}
	}

	for _, groupName := range groupNames() {
		if n := len("  ") + len(groupName) + len(":") + 2; n > column {
			column = n
		}
		for _, commandName := range groupCommandNames(groupName) {
			fullName := groupName + " " + commandName
			if n := len("    ") + len(fullName) + 2; n > column {
				column = n
			}
		}
	}

	return column
}

func writeGroupSummary(b *strings.Builder, groupName string, column int) {
	label := groupName + ":"
	desc := groupDescription(groupName)
	if desc == "" {
		fmt.Fprintf(b, "  %s\n", label)
		return
	}

	padding := column - len("  ") - len(label)
	if padding < 2 {
		padding = 2
	}
	fmt.Fprintf(b, "  %s%s%s\n", label, strings.Repeat(" ", padding), desc)
}

func commandDescription(name string) string {
	switch name {
	case "help":
		if _, found := app.commands["help"]; !found {
			return "Show help information."
		}
	case "version":
		if _, found := app.commands["version"]; !found {
			return "Show version information."
		}
	}

	cmd, found := app.commands[name]
	if !found {
		return ""
	}

	return cmd.description
}

func groupCommandDescription(groupName string, commandName string) string {
	group, found := app.groups[groupName]
	if !found {
		return ""
	}

	cmd, found := group.commands[commandName]
	if !found {
		return ""
	}

	return cmd.description
}

func groupDescription(name string) string {
	group, found := app.groups[name]
	if !found {
		return ""
	}

	return group.description
}
