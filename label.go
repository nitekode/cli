package cli

import (
	"strconv"
	"strings"
)

// label is a display-only grouping: its commands are ordinary root commands
// (invoked without a prefix) that render together under a title in help. Unlike
// a group it carries no invocation prefix, flag scope, or middleware.
type label struct {
	title    string
	commands []string
}

type LabelAdder interface {
	Command(sig string, description string, handler any, opts ...CommandOption)
}

type labelAdder struct {
	label *label
}

// Label registers a help heading. Commands added inside register at the root,
// exactly as cli.Command does, but render under title in help output.
func Label(title string, register func(LabelAdder)) {
	if strings.TrimSpace(title) == "" {
		panic("cli: label title cannot be empty")
	}
	for _, existing := range app.labels {
		if existing.title == title {
			panic("cli: duplicate label " + strconv.Quote(title))
		}
	}

	lbl := &label{title: title}
	app.labels = append(app.labels, lbl)
	register(labelAdder{label: lbl})
}

func (a labelAdder) Command(sig string, description string, handler any, opts ...CommandOption) {
	cmd, err := newCommand(sig, description, handler, opts...)
	if err != nil {
		panic("cli: " + err.Error())
	}
	if cmd.name == "" {
		panic("cli: commands under a label must have a name")
	}
	if _, exists := app.groups[cmd.name]; exists {
		panic("cli: command name conflicts with existing group " + strconv.Quote(cmd.name))
	}
	if _, exists := app.commands[cmd.name]; exists {
		panic("cli: duplicate command " + strconv.Quote(cmd.name))
	}
	if err := validateCommandFlags(&cmd); err != nil {
		panic("cli: " + err.Error())
	}

	app.commands[cmd.name] = cmd
	a.label.commands = append(a.label.commands, cmd.name)
}
