package cli

type CommandOption interface {
	applyCommand(*command)
}

type GroupOption interface {
	applyGroup(*group)
}

type hiddenOption struct{}

func Hidden() hiddenOption {
	return hiddenOption{}
}

func (hiddenOption) applyCommand(cmd *command) {
	cmd.hidden = true
}

func (hiddenOption) applyGroup(group *group) {
	group.hidden = true
}
