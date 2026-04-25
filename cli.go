package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

var app = struct {
	version string
	commit  string
	builtAt string

	in  io.Reader
	out io.Writer
	err io.Writer

	commands map[string]command
}{
	version: "dev",
	commit:  "unknown",
	builtAt: "unknown",

	in:  os.Stdin,
	out: os.Stdout,
	err: os.Stderr,

	commands: make(map[string]command),
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
	fmt.Fprintln(app.out, "this is how you use it")
	os.Exit(0)
}
