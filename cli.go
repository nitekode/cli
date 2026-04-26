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

	commands map[string]command
	groups   map[string]*group
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
	Command(sig string, handler any)
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

func Command(sig string, handler any) {
	cmd, err := newCommand(sig, handler)
	if err != nil {
		panic("cli: " + err.Error())
	}

	if _, exists := app.groups[cmd.name]; cmd.name != "" && exists {
		panic("cli: command name conflicts with existing group " + strconv.Quote(cmd.name))
	}

	app.commands[cmd.name] = cmd
}

func Group(name string, register func(GroupAdder)) {
	if err := validateGroupName(name); err != nil {
		panic("cli: " + err.Error())
	}
	if _, exists := app.commands[name]; exists {
		panic("cli: group name conflicts with existing command " + strconv.Quote(name))
	}
	if _, exists := app.groups[name]; exists {
		panic("cli: duplicate group " + strconv.Quote(name))
	}

	g := &group{
		name:     name,
		commands: make(map[string]command),
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
			return cmd.invoke(nil)
		}

		return errors.New("no command provided")
	}

	commandName := args[1]
	cmd, found := app.commands[commandName]
	if found {
		return cmd.invoke(args[2:])
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

		return groupCommand.invoke(args[3:])
	}

	root, hasRoot := app.commands[""]
	if !hasRoot {
		return fmt.Errorf("unknown command %q", commandName)
	}

	return root.invoke(args[1:])
}

func printUsageAndExit() {
	fmt.Fprint(app.out, globalHelp(filepath.Base(os.Args[0])))
	os.Exit(0)
}

func globalHelp(executable string) string {
	var b strings.Builder

	writeAppHeader(&b)
	b.WriteString("Usage:\n")

	if _, hasRoot := app.commands[""]; hasRoot {
		fmt.Fprintf(&b, "  %s [arguments]\n", executable)
	}

	if hasNamedCommands() {
		fmt.Fprintf(&b, "  %s {command} [arguments]\n", executable)
	}

	if hasNamedCommands() || len(app.groups) > 0 {
		b.WriteString("\nCommands:\n")
		for _, name := range commandNames() {
			fmt.Fprintf(&b, "  %s\n", name)
		}
		for _, groupName := range groupNames() {
			fmt.Fprintf(&b, "  %s\n", groupName)
			for _, commandName := range groupCommandNames(groupName) {
				fmt.Fprintf(&b, "    %s %s\n", groupName, commandName)
			}
		}
	}

	return b.String()
}

func groupHelp(executable string, group *group) string {
	var b strings.Builder

	writeAppHeader(&b)
	b.WriteString("Usage:\n")
	fmt.Fprintf(&b, "  %s {command} [arguments]\n", executable)

	if len(group.commands) > 0 {
		fmt.Fprintf(&b, "\nCommands in the %q group:\n", group.name)
		for _, commandName := range groupCommandNames(group.name) {
			fmt.Fprintf(&b, "  %s %s\n", group.name, commandName)
		}
	}

	return b.String()
}

func writeAppHeader(b *strings.Builder) {
	if app.name != "" {
		b.WriteString(app.name)
		if app.version != "" {
			b.WriteString(" ")
			b.WriteString(app.version)
		}
		b.WriteString("\n")
	}

	if app.description != "" {
		b.WriteString(app.description)
		b.WriteString("\n")
	}

	b.WriteString("\n")
}

func hasNamedCommands() bool {
	for name := range app.commands {
		if name != "" {
			return true
		}
	}

	return false
}

func commandNames() []string {
	names := make([]string, 0, len(app.commands))
	for name := range app.commands {
		if name == "" {
			continue
		}

		names = append(names, name)
	}

	slices.Sort(names)
	return names
}

func groupNames() []string {
	names := make([]string, 0, len(app.groups))
	for name := range app.groups {
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
		names = append(names, name)
	}

	slices.Sort(names)
	return names
}
