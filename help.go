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

type helpCommandsSectionData struct {
	CommandWidth      int
	GroupCommandWidth int
	Commands          []helpCommandSummary
	Groups            []helpGroupSection
}

var helpTemplateFuncs = template.FuncMap{
	"padRight": func(s string, width int) string {
		if len(s) >= width {
			return s
		}
		return s + strings.Repeat(" ", width-len(s))
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

//go:embed templates/help_usage.tmpl
var usageTemplateSource string

//go:embed templates/help_arguments.tmpl
var argumentsTemplateSource string

//go:embed templates/help_options.tmpl
var optionsTemplateSource string

//go:embed templates/help_commands.tmpl
var commandsTemplateSource string

var usageTemplate = template.Must(template.New("usage").Parse(usageTemplateSource))

var argumentsTemplate = template.Must(template.New("arguments").Parse(argumentsTemplateSource))

var optionsTemplate = template.Must(template.New("options").Funcs(helpTemplateFuncs).Parse(optionsTemplateSource))

var commandsTemplate = template.Must(template.New("commands").Funcs(helpTemplateFuncs).Parse(commandsTemplateSource))

func printUsageAndExit() {
	fmt.Fprint(app.out, globalHelp(filepath.Base(os.Args[0])))
	os.Exit(0)
}

func globalHelp(executable string) string {
	ensureInternalCommands()

	data := globalHelpData{
		Description: app.description,
		Usage:       make([]string, 0, 2),
		Options:     make([]helpCommandSummary, 0),
		Commands:    make([]helpCommandSummary, 0, len(app.commands)),
		Groups:      make([]helpGroupSection, 0, len(app.groups)),
	}

	if _, hasRoot := app.commands[""]; hasRoot {
		usage := executable + " [options] [arguments]"
		data.Usage = append(data.Usage, usage)
	}
	if hasNamedCommands() || len(app.groups) > 0 {
		usage := executable + " {command} [options] [arguments]"
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

	sections := make([]string, 0, 4)
	if data.Description != "" {
		sections = append(sections, data.Description)
	}
	sections = append(sections, renderHelpTemplate(usageTemplate, data.Usage))
	if len(data.Options) > 0 {
		sections = append(sections, renderHelpTemplate(optionsTemplate, data.Options))
	}
	if len(data.Commands) > 0 || len(data.Groups) > 0 {
		commandWidth, groupCommandWidth := commandSectionWidths(data.Commands, data.Groups)
		sections = append(sections, renderHelpTemplate(commandsTemplate, helpCommandsSectionData{
			CommandWidth:      commandWidth,
			GroupCommandWidth: groupCommandWidth,
			Commands:          data.Commands,
			Groups:            data.Groups,
		}))
	}

	return joinHelpSections(sections)
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

	sections := make([]string, 0, 4)
	if data.Description != "" {
		sections = append(sections, data.Description)
	}
	sections = append(sections, renderHelpTemplate(usageTemplate, []string{data.Usage}))
	if len(data.Arguments) > 0 {
		sections = append(sections, renderHelpTemplate(argumentsTemplate, data.Arguments))
	}
	if len(data.Options) > 0 {
		sections = append(sections, renderHelpTemplate(optionsTemplate, data.Options))
	}

	return joinHelpSections(sections)
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

	sections := make([]string, 0, 4)
	if data.Description != "" {
		sections = append(sections, data.Description)
	}
	sections = append(sections, renderHelpTemplate(usageTemplate, []string{data.Usage}))
	if len(data.Options) > 0 {
		sections = append(sections, renderHelpTemplate(optionsTemplate, data.Options))
	}
	if len(data.Commands) > 0 {
		commandWidth := 0
		for _, command := range data.Commands {
			if len(command.Label) > commandWidth {
				commandWidth = len(command.Label)
			}
		}
		sections = append(sections, renderHelpTemplate(commandsTemplate, helpCommandsSectionData{
			CommandWidth:      commandWidth,
			GroupCommandWidth: commandWidth,
			Commands:          data.Commands,
		}))
	}

	return joinHelpSections(sections)
}

func commandSectionWidths(commands []helpCommandSummary, groups []helpGroupSection) (int, int) {
	column := 0
	groupColumn := 0

	for _, command := range commands {
		if n := len("  ") + len(command.Label) + 2; n > column {
			column = n
		}
	}
	for _, group := range groups {
		if n := len("  ") + len(group.Label) + 2; n > column {
			column = n
		}
		for _, command := range group.Commands {
			if n := len("    ") + len(command.Label) + 2; n > column {
				column = n
			}
			if n := len(command.Label); n > groupColumn {
				groupColumn = n
			}
		}
	}

	if groupColumn == 0 {
		groupColumn = column - len("    ") - 2
	}

	return column - len("  ") - 2, groupColumn
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

func commandNames() []string {
	names := make([]string, 0, len(app.commands))
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
	return strings.TrimRight(b.String(), "\n")
}

func joinHelpSections(sections []string) string {
	filtered := make([]string, 0, len(sections))
	for _, section := range sections {
		if section == "" {
			continue
		}
		filtered = append(filtered, section)
	}

	if len(filtered) == 0 {
		return ""
	}

	return strings.Join(filtered, "\n\n") + "\n"
}

func commandDescription(name string) string {
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
