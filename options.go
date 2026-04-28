package cli

type CommandOption interface {
	applyCommand(*command)
}

type GroupOption interface {
	applyGroup(*group)
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
