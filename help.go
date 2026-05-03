package cli

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"text/template"
)

type usageError struct {
	body string
}

func (e usageError) Error() string {
	return e.body
}

type helpCommandSummary struct {
	Label       string
	Description string
}

type helpGroupSection struct {
	Label       string
	Description string
	Commands    []helpCommandSummary
}

type globalHelpData struct {
	Description string
	Usage       []string
	Options     []helpCommandSummary
	Commands    []helpCommandSummary
	Groups      []helpGroupSection
}

type commandHelpArgument struct {
	Name        string
	Description string
	Default     string
}

type commandHelpData struct {
	Description string
	Usage       string
	Options     []helpCommandSummary
	Arguments   []commandHelpArgument
}

type groupHelpData struct {
	Description string
	Usage       string
	Options     []helpCommandSummary
	Commands    []helpCommandSummary
}

var helpTemplateFuncs = template.FuncMap{
	"padRight": func(s string, width int) string {
		if len(s) >= width {
			return s
		}
		return s + strings.Repeat(" ", width-len(s))
	},
	"globalHelpCommandWidth": func(data globalHelpData) int {
		column := 0
		for _, command := range data.Commands {
			if n := len("  ") + len(command.Label) + 2; n > column {
				column = n
			}
		}
		for _, group := range data.Groups {
			if n := len("  ") + len(group.Label) + 2; n > column {
				column = n
			}
			for _, command := range group.Commands {
				if n := len("    ") + len(command.Label) + 2; n > column {
					column = n
				}
			}
		}

		return column - len("  ") - 2
	},
	"globalHelpGroupCommandWidth": func(data globalHelpData) int {
		width := 0
		for _, group := range data.Groups {
			for _, command := range group.Commands {
				if n := len("    ") + len(command.Label); n > width {
					width = n
				}
			}
		}
		column := 0
		for _, command := range data.Commands {
			if n := len("  ") + len(command.Label) + 2; n > column {
				column = n
			}
		}
		for _, group := range data.Groups {
			if n := len("  ") + len(group.Label) + 2; n > column {
				column = n
			}
			for _, command := range group.Commands {
				if n := len("    ") + len(command.Label) + 2; n > column {
					column = n
				}
			}
		}

		return column - len("    ") - 2
	},
	"groupHelpCommandWidth": func(commands []helpCommandSummary) int {
		width := 0
		for _, command := range commands {
			if len(command.Label) > width {
				width = len(command.Label)
			}
		}
		return width
	},
}

//go:embed templates/help_global.tmpl
var globalHelpTemplateSource string

//go:embed templates/help_command.tmpl
var commandHelpTemplateSource string

//go:embed templates/help_group.tmpl
var groupHelpTemplateSource string

var globalHelpTemplate = template.Must(template.New("globalHelp").Funcs(helpTemplateFuncs).Parse(
	globalHelpTemplateSource))

var commandHelpTemplate = template.Must(template.New("commandHelp").Funcs(helpTemplateFuncs).Parse(
	commandHelpTemplateSource))

var groupHelpTemplate = template.Must(template.New("groupHelp").Funcs(helpTemplateFuncs).Parse(
	groupHelpTemplateSource))

func printUsageAndExit() {
	fmt.Fprint(app.out, globalHelp(filepath.Base(os.Args[0])))
	os.Exit(0)
}

func globalHelp(executable string) string {
	data := globalHelpData{
		Description: app.description,
		Usage:       make([]string, 0, 2),
		Options:     make([]helpCommandSummary, 0),
		Commands:    make([]helpCommandSummary, 0, len(app.commands)),
		Groups:      make([]helpGroupSection, 0, len(app.groups)),
	}

	if _, hasRoot := app.commands[""]; hasRoot {
		usage := executable
		if app.flags != nil {
			usage += " [options]"
		}
		usage += " [arguments]"
		data.Usage = append(data.Usage, usage)
	}
	if hasNamedCommands() || len(app.groups) > 0 {
		usage := executable + " {command}"
		if app.flags != nil {
			usage += " [options]"
		}
		usage += " [arguments]"
		data.Usage = append(data.Usage, usage)
	}

	if app.flags != nil {
		for _, field := range app.flags.fields {
			desc := field.Description
			if field.Default != "" {
				if desc != "" {
					desc += " "
				}
				desc += "(default=" + field.Default + ")"
			}
			data.Options = append(data.Options, helpCommandSummary{
				Label:       formatOptionLabel(field),
				Description: desc,
			})
		}
	}

	for _, name := range commandNames() {
		data.Commands = append(data.Commands, helpCommandSummary{
			Label:       name,
			Description: commandDescription(name),
		})
	}

	for _, groupName := range groupNames() {
		section := helpGroupSection{
			Label:       groupName + ":",
			Description: groupDescription(groupName),
			Commands:    make([]helpCommandSummary, 0, len(groupCommandNames(groupName))),
		}
		for _, commandName := range groupCommandNames(groupName) {
			section.Commands = append(section.Commands, helpCommandSummary{
				Label:       groupName + " " + commandName,
				Description: groupCommandDescription(groupName, commandName),
			})
		}
		data.Groups = append(data.Groups, section)
	}

	return renderHelpTemplate(globalHelpTemplate, data)
}

func commandHelp(executable string, cmd command) string {
	data := commandHelpData{
		Description: cmd.description,
		Usage:       commandUsage(executable, cmd),
		Options:     make([]helpCommandSummary, 0),
		Arguments:   make([]commandHelpArgument, 0, len(cmd.arguments)),
	}
	if cmd.flags != nil && len(cmd.flags.fields) > 0 {
		data.Usage = commandUsageWithOptions(executable, cmd)
		for _, field := range cmd.flags.fields {
			desc := field.Description
			if field.Default != "" {
				if desc != "" {
					desc += " "
				}
				desc += "(default=" + field.Default + ")"
			}
			data.Options = append(data.Options, helpCommandSummary{
				Label:       formatOptionLabel(field),
				Description: desc,
			})
		}
	}

	for _, arg := range cmd.arguments {
		item := commandHelpArgument{
			Name:        arg.Name,
			Description: arg.Description,
		}
		if arg.Kind == defaultArgument {
			item.Default = arg.Default
		}
		data.Arguments = append(data.Arguments, item)
	}

	return renderHelpTemplate(commandHelpTemplate, data)
}

func groupHelp(executable string, group *group) string {
	data := groupHelpData{
		Description: group.description,
		Usage:       executable + " " + group.name + " {command} [arguments]",
		Options:     make([]helpCommandSummary, 0),
		Commands:    make([]helpCommandSummary, 0, len(group.commands)),
	}
	if group.flags != nil && len(group.flags.fields) > 0 {
		data.Usage = executable + " " + group.name + " {command} [options] [arguments]"
		for _, field := range group.flags.fields {
			desc := field.Description
			if field.Default != "" {
				if desc != "" {
					desc += " "
				}
				desc += "(default=" + field.Default + ")"
			}
			data.Options = append(data.Options, helpCommandSummary{
				Label:       formatOptionLabel(field),
				Description: desc,
			})
		}
	}

	for _, commandName := range groupCommandNames(group.name) {
		data.Commands = append(data.Commands, helpCommandSummary{
			Label:       commandName,
			Description: groupCommandDescription(group.name, commandName),
		})
	}

	return renderHelpTemplate(groupHelpTemplate, data)
}

func formatOptionLabel(field flagField) string {
	label := "--" + field.Name
	if field.Bool {
		return label
	}

	placeholder := strings.ToUpper(strings.ReplaceAll(field.Name, "-", "_"))
	return label + "[=" + placeholder + "]"
}

func commandUsage(executable string, cmd command) string {
	return cmd.usage(executable)
}

func commandUsageWithOptions(executable string, cmd command) string {
	parts := make([]string, 0, len(cmd.arguments)+3)
	parts = append(parts, executable)
	if cmd.name != "" {
		parts = append(parts, cmd.name)
	}
	parts = append(parts, "[options]")
	for _, arg := range cmd.arguments {
		parts = append(parts, formatUsageArgument(arg))
	}
	return strings.Join(parts, " ")
}

func runHelp(args []string) error {
	executable := filepath.Base(args[0])
	switch len(args) {
	case 2:
		fmt.Fprint(app.out, globalHelp(executable))
		return nil
	case 3:
		name := args[2]
		if cmd, found := app.commands[name]; found {
			fmt.Fprint(app.out, commandHelp(executable, cmd))
			return nil
		}
		if group, found := app.groups[name]; found {
			fmt.Fprint(app.out, groupHelp(executable, group))
			return nil
		}
		if name == "help" {
			fmt.Fprintf(app.out, "Usage:\n  %s help [command]\n", executable)
			return nil
		}
		if name == "version" {
			fmt.Fprintf(app.out, "Usage:\n  %s version\n", executable)
			return nil
		}
	default:
		groupName := args[2]
		commandName := args[3]
		if group, found := app.groups[groupName]; found {
			if cmd, found := group.commands[commandName]; found {
				fmt.Fprint(app.out, commandHelp(executable, cmd))
				return nil
			}
		}
	}

	return fmt.Errorf("unknown help topic %q", strings.Join(args[2:], " "))
}

func commandNames() []string {
	names := make([]string, 0, len(app.commands))
	if hasAutoBuiltInCommands() {
		if _, found := app.commands["help"]; !found {
			names = append(names, "help")
		}
		if _, found := app.commands["version"]; !found {
			names = append(names, "version")
		}
	}

	for name := range app.commands {
		if name == "" || app.commands[name].hidden {
			continue
		}

		names = append(names, name)
	}

	slices.Sort(names)
	return names
}

func groupNames() []string {
	names := make([]string, 0, len(app.groups))
	for name := range app.groups {
		if app.groups[name].hidden {
			continue
		}
		names = append(names, name)
	}

	slices.Sort(names)
	return names
}

func groupCommandNames(groupName string) []string {
	group, found := app.groups[groupName]
	if !found {
		return nil
	}

	names := make([]string, 0, len(group.commands))
	for name := range group.commands {
		if group.commands[name].hidden {
			continue
		}
		names = append(names, name)
	}

	slices.Sort(names)
	return names
}

func renderHelpTemplate(tmpl *template.Template, data any) string {
	var b bytes.Buffer
	if err := tmpl.Execute(&b, data); err != nil {
		panic("cli: failed to render help template: " + err.Error())
	}
	return b.String()
}

func commandDescription(name string) string {
	switch name {
	case "help":
		if _, found := app.commands["help"]; !found {
			return "Show help information."
		}
	case "version":
		if _, found := app.commands["version"]; !found {
			return "Show version information."
		}
	}

	cmd, found := app.commands[name]
	if !found {
		return ""
	}

	return cmd.description
}

func groupCommandDescription(groupName string, commandName string) string {
	group, found := app.groups[groupName]
	if !found {
		return ""
	}

	cmd, found := group.commands[commandName]
	if !found {
		return ""
	}

	return cmd.description
}

func groupDescription(name string) string {
	group, found := app.groups[name]
	if !found {
		return ""
	}

	return group.description
}
