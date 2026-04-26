package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

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
}{
	name: filepath.Base(os.Args[0]),

	version: "dev",
	commit:  "unknown",
	builtAt: "unknown",

	in:  os.Stdin,
	out: os.Stdout,
	err: os.Stderr,

	commands: make(map[string]command),
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

	app.commands[cmd.name] = cmd
}

func Run() {
	if len(os.Args) <= 1 && app.commands[""].handlerType == nil {
		printUsageAndExit()
	}

	if err := RunWith(os.Args); err != nil {
		executable := filepath.Base(os.Args[0])
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
	if !found {
		root, hasRoot := app.commands[""]
		if !hasRoot {
			return fmt.Errorf("unknown command %q", commandName)
		}

		return root.invoke(args[1:])
	}

	return cmd.invoke(args[2:])
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

	if names := commandNames(); len(names) > 0 {
		b.WriteString("\nCommands:\n")
		for _, name := range names {
			fmt.Fprintf(&b, "  %s\n", name)
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
