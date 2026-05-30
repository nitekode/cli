package cli

type CommandOption interface {
	applyCommand(*command)
}

type GroupOption interface {
	applyGroup(*group)
}

func setArgumentDescription(cmd *command, name string, description string) {
	for i := range cmd.arguments {
		if cmd.arguments[i].Name == name {
			cmd.arguments[i].Description = description
			return
		}
	}

	panic("cli: unknown argument " + name)
}

// Middleware

type middlewareOption struct {
	middleware []MiddlewareFunc
}

func Middleware(middleware ...MiddlewareFunc) middlewareOption {
	return middlewareOption{middleware: middleware}
}
func (o middlewareOption) applyCommand(cmd *command) {
	cmd.middleware = append(cmd.middleware, o.middleware...)
}

func (o middlewareOption) applyGroup(group *group) {
	group.middleware = append(group.middleware, o.middleware...)
}

// Hidden

type hiddenOption struct{}

func Hidden() hiddenOption { return hiddenOption{} }

func (hiddenOption) applyCommand(cmd *command) { cmd.hidden = true }
func (hiddenOption) applyGroup(group *group)   { group.hidden = true }

// RawArgs

type rawArgsOption struct{}

// RawArgs marks a command as a pass-through: every argument after the command
// name is forwarded to the handler verbatim, without parsing flags, options, or
// the "--" terminator. The handler must be func(...string) error.
func RawArgs() rawArgsOption { return rawArgsOption{} }

func (rawArgsOption) applyCommand(cmd *command) { cmd.rawArgs = true }

// ArgDesc

type argDescOption struct {
	name        string
	description string
}

func ArgDesc(name string, description string) argDescOption {
	return argDescOption{
		name:        name,
		description: description,
	}
}

func (o argDescOption) applyCommand(cmd *command) {
	for i := range cmd.arguments {
		if cmd.arguments[i].Name == o.name {
			cmd.arguments[i].Description = o.description
			return
		}
	}

	panic("cli: unknown argument " + o.name)
}

// Flags

type flagsOption struct {
	flags *flagSet
}

func (o flagsOption) applyCommand(cmd *command) {
	cmd.flags = o.flags
}

func (o flagsOption) applyGroup(group *group) {
	group.flags = o.flags
}
