package cli

type group struct {
	description string
	name        string
	commands    map[string]command
	flags       *flagSet
	hidden      bool
	hiddenWhen  func() bool
	middleware  []MiddlewareFunc
}

func (g *group) isHidden() bool {
	return g.hidden || (g.hiddenWhen != nil && g.hiddenWhen())
}

type groupAdder struct {
	group *group
}

func (g groupAdder) Command(sig string, description string, handler any, opts ...CommandOption) {
	cmd, err := newCommand(sig, description, handler, opts...)
	if err != nil {
		panic("cli: " + err.Error())
	}
	if cmd.name == "" {
		panic("cli: grouped commands must have a command name")
	}
	if _, exists := g.group.commands[cmd.name]; exists {
		panic("cli: duplicate command " + cmd.name + " in group " + g.group.name)
	}

	cmd.group = g.group
	cmd.name = g.group.name + " " + cmd.name
	if err := validateCommandFlags(&cmd); err != nil {
		panic("cli: " + err.Error())
	}
	g.group.commands[commandLeafName(cmd.name)] = cmd
}
