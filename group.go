package cli

type group struct {
	name     string
	commands map[string]command
	hidden   bool
}

type groupAdder struct {
	group *group
}

func (g groupAdder) Command(sig string, handler any, opts ...CommandOption) {
	cmd, err := newCommand(sig, handler, opts...)
	if err != nil {
		panic("cli: " + err.Error())
	}
	if cmd.name == "" {
		panic("cli: grouped commands must have a command name")
	}
	if _, exists := g.group.commands[cmd.name]; exists {
		panic("cli: duplicate command " + cmd.name + " in group " + g.group.name)
	}

	cmd.name = g.group.name + " " + cmd.name
	g.group.commands[commandLeafName(cmd.name)] = cmd
}
