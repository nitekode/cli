package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestRunWithGroupWithoutCommandReturnsGroupHelp(t *testing.T) {
	originalCommands := app.commands
	originalGroups := app.groups
	originalName := app.name
	originalDescription := app.description
	originalVersion := app.version
	app.commands = make(map[string]command)
	app.groups = make(map[string]*group)
	app.name = "myapp"
	app.description = "Example app"
	app.version = "1.2.3"
	defer func() {
		app.commands = originalCommands
		app.groups = originalGroups
		app.name = originalName
		app.description = originalDescription
		app.version = originalVersion
	}()

	Group("calc", "Calculator commands.", func(g GroupAdder) {
		g.Command("add {a} {b}", "Add two numbers.", func(a string, b string) error { return nil })
		g.Command("sub {a} {b}", "Subtract two numbers.", func(a string, b string) error { return nil })
	})

	err := RunWith([]string{"myapp", "calc"})
	var usage usageError
	if !errors.As(err, &usage) {
		t.Fatalf("RunWith error = %v, want usageError", err)
	}

	for _, want := range []string{
		"Calculator commands.\n",
		"Usage:",
		"  myapp calc {command} [arguments]",
		"Commands:",
		"  add",
		"  sub",
	} {
		if !strings.Contains(usage.body, want) {
			t.Fatalf("group help missing %q in:\n%s", want, usage.body)
		}
	}

	if strings.Contains(usage.body, "\n  calc\n") {
		t.Fatalf("group help should not include standalone group header:\n%s", usage.body)
	}
}

func TestRunWithHelpForGroup(t *testing.T) {
	originalCommands := app.commands
	originalGroups := app.groups
	originalOut := app.out
	app.commands = make(map[string]command)
	app.groups = make(map[string]*group)
	var out bytes.Buffer
	app.out = &out
	defer func() {
		app.commands = originalCommands
		app.groups = originalGroups
		app.out = originalOut
	}()

	Group("calc", "Calculator commands.", func(g GroupAdder) {
		g.Command("add {a} {b}", "Add two numbers.", func(a string, b string) error { return nil })
	})

	if err := RunWith([]string{"myapp", "help", "calc"}); err != nil {
		t.Fatalf("RunWith returned error: %v", err)
	}

	for _, want := range []string{
		"Calculator commands.\n",
		"Usage:",
		"  myapp calc {command} [arguments]",
		"Commands:",
		"  add",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("help output missing %q in:\n%s", want, out.String())
		}
	}
}

func TestRunWithHelpForCommand(t *testing.T) {
	originalCommands := app.commands
	originalGroups := app.groups
	originalOut := app.out
	app.commands = make(map[string]command)
	app.groups = make(map[string]*group)
	var out bytes.Buffer
	app.out = &out
	defer func() {
		app.commands = originalCommands
		app.groups = originalGroups
		app.out = originalOut
	}()

	Command("greet {name}", "Greet someone.", func(name string) error { return nil }, ArgDesc("name", "The person to greet."))

	if err := RunWith([]string{"myapp", "help", "greet"}); err != nil {
		t.Fatalf("RunWith returned error: %v", err)
	}

	if !strings.Contains(out.String(), "Greet someone.\n\n") {
		t.Fatalf("help output = %q", out.String())
	}
	if !strings.Contains(out.String(), "  myapp greet <name>\n") {
		t.Fatalf("help output = %q", out.String())
	}
	if !strings.Contains(out.String(), "\nArguments:\n  name  The person to greet.\n") {
		t.Fatalf("help output = %q", out.String())
	}
}

func TestRunWithHelpForGroupedCommand(t *testing.T) {
	originalCommands := app.commands
	originalGroups := app.groups
	originalOut := app.out
	app.commands = make(map[string]command)
	app.groups = make(map[string]*group)
	var out bytes.Buffer
	app.out = &out
	defer func() {
		app.commands = originalCommands
		app.groups = originalGroups
		app.out = originalOut
	}()

	Group("calc", "Calculator commands.", func(g GroupAdder) {
		g.Command("add {a:first number} {b:second number}", "Add two numbers.", func(a string, b string) error { return nil })
	})

	if err := RunWith([]string{"myapp", "help", "calc", "add"}); err != nil {
		t.Fatalf("RunWith returned error: %v", err)
	}

	if !strings.Contains(out.String(), "Add two numbers.\n\n") {
		t.Fatalf("help output = %q", out.String())
	}
	if !strings.Contains(out.String(), "  myapp calc add <a> <b>\n") {
		t.Fatalf("help output = %q", out.String())
	}
	for _, want := range []string{
		"\nArguments:\n",
		"  a  first number\n",
		"  b  second number\n",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("help output missing %q in:\n%s", want, out.String())
		}
	}
}

func TestRunWithHelpForCommandShowsArgumentDefault(t *testing.T) {
	originalCommands := app.commands
	originalGroups := app.groups
	originalOut := app.out
	app.commands = make(map[string]command)
	app.groups = make(map[string]*group)
	var out bytes.Buffer
	app.out = &out
	defer func() {
		app.commands = originalCommands
		app.groups = originalGroups
		app.out = originalOut
	}()

	Command("sleep {duration=1}", "Sleep for a second", func(duration string) error { return nil }, ArgDesc("duration", "sleep time in seconds"))

	if err := RunWith([]string{"myapp", "help", "sleep"}); err != nil {
		t.Fatalf("RunWith returned error: %v", err)
	}

	if !strings.Contains(out.String(), "  duration  sleep time in seconds (default=1)\n") {
		t.Fatalf("help output = %q", out.String())
	}
}

func TestRunWithUnknownGroupCommandReturnsGroupHelp(t *testing.T) {
	originalCommands := app.commands
	originalGroups := app.groups
	app.commands = make(map[string]command)
	app.groups = make(map[string]*group)
	defer func() {
		app.commands = originalCommands
		app.groups = originalGroups
	}()

	Group("calc", "Calculator commands.", func(g GroupAdder) {
		g.Command("add {a} {b}", "Add two numbers.", func(a string, b string) error { return nil })
	})

	err := RunWith([]string{"myapp", "calc", "mul"})
	var usage usageError
	if !errors.As(err, &usage) {
		t.Fatalf("RunWith error = %v, want usageError", err)
	}

	if !strings.Contains(usage.body, "  myapp calc {command} [arguments]") {
		t.Fatalf("group help = %q", usage.body)
	}
}

func TestGlobalHelp(t *testing.T) {
	originalName := app.name
	originalDescription := app.description
	originalVersion := app.version
	originalCommands := app.commands
	originalGroups := app.groups

	app.name = "myapp"
	app.description = "Example app"
	app.version = "1.2.3"
	app.commands = make(map[string]command)
	app.groups = make(map[string]*group)
	defer func() {
		app.name = originalName
		app.description = originalDescription
		app.version = originalVersion
		app.commands = originalCommands
		app.groups = originalGroups
	}()

	Command("{file}", "Read a file.", func(file *string) error { return nil })
	Command("greet {name}", "Greet someone.", func(name string) error { return nil })
	Command("version", "Show version.", func() error { return nil })
	Group("secret", "Secret commands.", func(g GroupAdder) {
		g.Command("add {name}", "Add a secret.", func(name string) error { return nil })
		g.Command("edit {name}", "Edit a secret", func(name string) error { return nil })
		g.Command("delete {name}", "Delete a secret", func(name string) error { return nil })
	})

	got := globalHelp("myapp")

	for _, want := range []string{
		"Example app",
		"Usage:",
		"  myapp [options] [arguments]",
		"  myapp {command} [options] [arguments]",
		"Commands:",
		"  greet            Greet someone.",
		"  help             Show help information.",
		"  secret:          Secret commands.",
		"    secret add     Add a secret.",
		"    secret delete  Delete a secret",
		"    secret edit    Edit a secret",
		"  version          Show version.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("globalHelp missing %q in:\n%s", want, got)
		}
	}
}

func TestGlobalHelpOmitsHiddenCommandAndGroup(t *testing.T) {
	originalCommands := app.commands
	originalGroups := app.groups
	app.commands = make(map[string]command)
	app.groups = make(map[string]*group)
	defer func() {
		app.commands = originalCommands
		app.groups = originalGroups
	}()

	Command("visible", "Visible command.", func() error { return nil })
	Command("hidden", "Hidden command.", func() error { return nil }, Hidden())
	Group("secret", "Secret commands.", func(g GroupAdder) {
		g.Command("add", "Add a secret.", func() error { return nil })
	}, Hidden())

	got := globalHelp("myapp")

	if strings.Contains(got, "\n  hidden\n") {
		t.Fatalf("globalHelp should omit hidden command:\n%s", got)
	}
	if strings.Contains(got, "\n  secret\n") || strings.Contains(got, "secret add") {
		t.Fatalf("globalHelp should omit hidden group:\n%s", got)
	}
	if !strings.Contains(got, "\n  visible  Visible command.\n") {
		t.Fatalf("globalHelp should include visible command:\n%s", got)
	}
}

func TestPrintUsageAndExitWritesGlobalHelp(t *testing.T) {
	originalOut := app.out
	originalCommands := app.commands
	originalGroups := app.groups
	originalName := app.name
	originalDescription := app.description
	originalVersion := app.version

	var out bytes.Buffer
	app.out = &out
	app.commands = make(map[string]command)
	app.groups = make(map[string]*group)
	app.name = "myapp"
	app.description = "Example app"
	app.version = "1.2.3"
	defer func() {
		app.out = originalOut
		app.commands = originalCommands
		app.groups = originalGroups
		app.name = originalName
		app.description = originalDescription
		app.version = originalVersion
	}()

	Command("greet {name}", "Greet someone.", func(name string) error { return nil })

	got := globalHelp("myapp")
	if !strings.Contains(got, "  myapp {command} [options] [arguments]") {
		t.Fatalf("globalHelp = %q", got)
	}
}
