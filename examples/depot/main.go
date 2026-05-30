package main

import (
	"fmt"

	"github.com/nitekode/cli"
)

type flags struct {
	Verbose bool `flag:"verbose,v" desc:"print detailed output"`
}

func main() {
	cli.Name("depot")
	cli.Description("Manage services and their images.")
	cli.Version("1.0.0")
	cli.GlobalFlags[flags]()

	// A plain top-level command. It renders under the default "Commands:"
	// heading alongside the built-in help and version commands.
	cli.Command("doctor", "Check the environment for problems.", func(f flags) error {
		fmt.Fprintln(cli.Out(), "everything looks healthy")
		return nil
	})

	// Labels are display-only: these commands are invoked without a prefix
	// (depot up web, depot status) but render together under a heading in help.
	cli.Label("Services", func(l cli.LabelAdder) {
		l.Command("up {service}", "Start a service.", serviceAction("starting"))
		l.Command("down {service}", "Stop a service.", serviceAction("stopping"))
		l.Command("status", "Show service status.", func(f flags) error {
			fmt.Fprintln(cli.Out(), "all services running")
			return nil
		})
	})

	cli.Label("Images", func(l cli.LabelAdder) {
		l.Command("build {service}", "Build a service image.", serviceAction("building"))
		l.Command("pull {service}", "Pull a service image.", serviceAction("pulling"))
	})

	// A group is a real namespace: invoked with the prefix (depot config get).
	cli.Group("config", "Read and write configuration.", func(g cli.GroupAdder) {
		g.Command("get {key}", "Show a configuration value.", func(key string) error {
			fmt.Fprintf(cli.Out(), "%s = (unset)\n", key)
			return nil
		})
		g.Command("set {key} {value}", "Update a configuration value.", func(key string, value string) error {
			fmt.Fprintf(cli.Out(), "set %s = %s\n", key, value)
			return nil
		})
	})

	cli.Run()
}

func serviceAction(verb string) func(flags, string) error {
	return func(f flags, service string) error {
		fmt.Fprintf(cli.Out(), "%s %s\n", verb, service)
		if f.Verbose {
			fmt.Fprintln(cli.Out(), "(done)")
		}
		return nil
	}
}
