package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var app = struct {
	name        string
	description string
	executable  string

	version string
	commit  string
	builtAt string

	in  io.Reader
	out io.Writer
	err io.Writer

	commands   map[string]command
	flags      *flagSet
	groups     map[string]*group
	middleware []MiddlewareFunc
}{
	name:       filepath.Base(os.Args[0]),
	executable: filepath.Base(os.Args[0]),

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
	if err := validateCommandFlags(&cmd); err != nil {
		panic("cli: " + err.Error())
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
	if err := validateGroupFlags(g); err != nil {
		panic("cli: " + err.Error())
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
	ensureInternalCommands()
	if len(args) > 0 {
		app.executable = filepath.Base(args[0])
	}

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

func printVersion() {
	fmt.Fprintln(app.out, versionString())
}

func ensureInternalCommands() {
	ensureInternalCommand("help {topic}", "Show help information.", helpHandler)
	ensureInternalCommand("version", "Show version information.", versionHandler)
}

func ensureInternalCommand(sig string, description string, handler any) {
	name := strings.Fields(sig)[0]
	if _, exists := app.commands[name]; exists {
		return
	}
	if _, exists := app.groups[name]; exists {
		return
	}

	cmd, err := newCommand(sig, description, handler)
	if err != nil {
		panic("cli: " + err.Error())
	}
	if err := validateCommandFlags(&cmd); err != nil {
		panic("cli: " + err.Error())
	}

	app.commands[cmd.name] = cmd
}

func helpHandler(topic ...string) error {
	switch len(topic) {
	case 0:
		fmt.Fprint(app.out, globalHelp(app.executable))
		return nil
	case 1:
		name := topic[0]
		if cmd, found := app.commands[name]; found {
			fmt.Fprint(app.out, commandHelp(app.executable, cmd))
			return nil
		}
		if group, found := app.groups[name]; found {
			fmt.Fprint(app.out, groupHelp(app.executable, group))
			return nil
		}
	case 2:
		groupName := topic[0]
		commandName := topic[1]
		if group, found := app.groups[groupName]; found {
			if cmd, found := group.commands[commandName]; found {
				fmt.Fprint(app.out, commandHelp(app.executable, cmd))
				return nil
			}
		}
	}

	return fmt.Errorf("unknown help topic %q", strings.Join(topic, " "))
}

func versionHandler() error {
	printVersion()
	return nil
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

func hasNamedCommands() bool {
	for name, cmd := range app.commands {
		if name != "" && !cmd.hidden {
			return true
		}
	}

	return false
}
