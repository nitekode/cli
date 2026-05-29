package main

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/nitekode/cli"
)

type globalFlags struct {
	Repeat int `flag:"repeat,r" default:"1"`
}

type formatterFlags struct {
	globalFlags
	Prefix string `flag:"prefix,p"`
}

type upperFlags struct {
	formatterFlags
	Strong bool `flag:"strong,s"`
}

func main() {
	cli.GlobalFlags[globalFlags]()
	cli.Group("format", "Format a text string", formatGroup, cli.Flags[formatterFlags]())
	cli.Run()
}

func formatGroup(ga cli.GroupAdder) {
	ga.Command("upper {text}", "Uppercase text", upperHandler,
		cli.Flags[upperFlags](),
	)
	ga.Command("lower {text}", "Lowercase text", lowerHandler,
		cli.Flags[upperFlags](),
	)
}

func upperHandler(flags upperFlags, text string) error {
	for i := 0; i < flags.globalFlags.Repeat; i++ {
		cli.Out().Write([]byte(transform(text, strings.ToUpper, flags.Prefix, flags.Strong)))
	}

	return nil
}

func lowerHandler(flags upperFlags, text string) error {
	for i := 0; i < flags.globalFlags.Repeat; i++ {
		cli.Out().Write([]byte(transform(text, strings.ToLower, flags.Prefix, flags.Strong)))
	}

	return nil
}

func transform(s string, t func(string) string, prefix string, strong bool) string {
	b := bytes.Buffer{}

	fmt.Fprintf(&b, "%s%s", prefix, t(s))
	if strong {
		fmt.Fprint(&b, "!")
	}
	fmt.Fprintln(&b)

	return b.String()
}
