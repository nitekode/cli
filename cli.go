package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type CommandHandler func() error

type command struct {
	handler CommandHandler
}

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

func SetIn(in io.Reader)   { app.in = in }
func SetOut(out io.Writer) { app.out = out }
func SetErr(err io.Writer) { app.err = err }

func In() io.Reader  { return app.in }
func Out() io.Writer { return app.out }
func Err() io.Writer { return app.err }

func Command(name string, handler CommandHandler) {
	app.commands[name] = command{
		handler: handler,
	}
}

func Run() {
	executable := filepath.Base(os.Args[0])

	if len(os.Args) == 1 {
		// No command has been given so print usage
		printUsageAndExit()
	}

	// Check that the command is available
	cmd, found := app.commands[os.Args[1]]
	if !found {
		fmt.Printf("%s %s: unknown command\n", executable, os.Args[1])
		os.Exit(1)
	}

	// Run command
	if err := cmd.handler(); err != nil {
		os.Exit(2)
	}
}

func printUsageAndExit() {
	fmt.Println("this is how you use it")
	os.Exit(0)
}
