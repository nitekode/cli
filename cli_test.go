package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestRunWith(t *testing.T) {
	originalCommands := app.commands
	originalGroups := app.groups
	app.commands = make(map[string]command)
	app.groups = make(map[string]*group)
	defer func() {
		app.commands = originalCommands
		app.groups = originalGroups
	}()

	Command("greet {name} {title=friend} {suffix} {others}", func(name string, title string, suffix *string, others ...string) error {
		return nil
	})

	if err := RunWith([]string{"test"}); err == nil || !strings.Contains(err.Error(), "no command provided") {
		t.Fatalf("RunWith no command error = %v", err)
	}

	if err := RunWith([]string{"test", "missing"}); err == nil || !strings.Contains(err.Error(), `unknown command "missing"`) {
		t.Fatalf("RunWith unknown command error = %v", err)
	}

	if err := RunWith([]string{"test", "greet", "alice"}); err != nil {
		t.Fatalf("RunWith returned error: %v", err)
	}

	app.commands = make(map[string]command)
	app.groups = make(map[string]*group)

	var gotPath *string
	Command("{path}", func(path *string) error {
		gotPath = path
		return nil
	})

	if err := RunWith([]string{"test"}); err != nil {
		t.Fatalf("RunWith root no args returned error: %v", err)
	}

	if gotPath != nil {
		t.Fatalf("gotPath = %v, want nil", *gotPath)
	}

	if err := RunWith([]string{"test", "file.txt"}); err != nil {
		t.Fatalf("RunWith root arg returned error: %v", err)
	}

	if gotPath == nil || *gotPath != "file.txt" {
		t.Fatalf("gotPath = %v, want %q", gotPath, "file.txt")
	}
}

func TestRunWithRootOptionalNoArgs(t *testing.T) {
	originalCommands := app.commands
	originalGroups := app.groups
	app.commands = make(map[string]command)
	app.groups = make(map[string]*group)
	defer func() {
		app.commands = originalCommands
		app.groups = originalGroups
	}()

	var gotPath *string
	Command("{file}", func(file *string) error {
		gotPath = file
		return nil
	})

	if err := RunWith([]string{"test"}); err != nil {
		t.Fatalf("RunWith returned error: %v", err)
	}

	if gotPath != nil {
		t.Fatalf("gotPath = %v, want nil", *gotPath)
	}
}

func TestRunWithPrefersSubcommandOverRoot(t *testing.T) {
	originalCommands := app.commands
	originalGroups := app.groups
	app.commands = make(map[string]command)
	app.groups = make(map[string]*group)
	defer func() {
		app.commands = originalCommands
		app.groups = originalGroups
	}()

	var called string

	Command("{value}", func(value string) error {
		called = "root:" + value
		return nil
	})

	Command("greet {name}", func(name string) error {
		called = "sub:" + name
		return nil
	})

	if err := RunWith([]string{"test", "greet", "alice"}); err != nil {
		t.Fatalf("RunWith returned error: %v", err)
	}

	if called != "sub:alice" {
		t.Fatalf("called = %q, want %q", called, "sub:alice")
	}
}

func TestRunWithGroupCommand(t *testing.T) {
	originalCommands := app.commands
	originalGroups := app.groups
	app.commands = make(map[string]command)
	app.groups = make(map[string]*group)
	defer func() {
		app.commands = originalCommands
		app.groups = originalGroups
	}()

	var called string

	Group("secret", func(g GroupAdder) {
		g.Command("add {name}", func(name string) error {
			called = "add:" + name
			return nil
		})
	})

	if err := RunWith([]string{"test", "secret", "add", "api-key"}); err != nil {
		t.Fatalf("RunWith returned error: %v", err)
	}

	if called != "add:api-key" {
		t.Fatalf("called = %q, want %q", called, "add:api-key")
	}
}

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

	Group("calc", func(g GroupAdder) {
		g.Command("add {a} {b}", func(a string, b string) error { return nil })
		g.Command("sub {a} {b}", func(a string, b string) error { return nil })
	})

	err := RunWith([]string{"myapp", "calc"})
	var usage usageError
	if !errors.As(err, &usage) {
		t.Fatalf("RunWith error = %v, want usageError", err)
	}

	for _, want := range []string{
		"Usage:",
		"  myapp {command} [arguments]",
		`Commands in the "calc" group:`,
		"  calc add",
		"  calc sub",
	} {
		if !strings.Contains(usage.body, want) {
			t.Fatalf("group help missing %q in:\n%s", want, usage.body)
		}
	}

	if strings.Contains(usage.body, "\n  calc\n") {
		t.Fatalf("group help should not include standalone group header:\n%s", usage.body)
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

	Group("calc", func(g GroupAdder) {
		g.Command("add {a} {b}", func(a string, b string) error { return nil })
	})

	err := RunWith([]string{"myapp", "calc", "mul"})
	var usage usageError
	if !errors.As(err, &usage) {
		t.Fatalf("RunWith error = %v, want usageError", err)
	}

	if !strings.Contains(usage.body, "  myapp {command} [arguments]") {
		t.Fatalf("group help = %q", usage.body)
	}
}

func TestRunWithGroupTakesPrecedenceOverRoot(t *testing.T) {
	originalCommands := app.commands
	originalGroups := app.groups
	app.commands = make(map[string]command)
	app.groups = make(map[string]*group)
	defer func() {
		app.commands = originalCommands
		app.groups = originalGroups
	}()

	var called string

	Command("{value}", func(value string) error {
		called = "root:" + value
		return nil
	})

	Group("secret", func(g GroupAdder) {
		g.Command("add", func() error {
			called = "group:add"
			return nil
		})
	})

	if err := RunWith([]string{"test", "secret", "add"}); err != nil {
		t.Fatalf("RunWith returned error: %v", err)
	}

	if called != "group:add" {
		t.Fatalf("called = %q, want %q", called, "group:add")
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

	Command("{file}", func(file *string) error { return nil })
	Command("greet {name}", func(name string) error { return nil })
	Command("version", func() error { return nil })
	Group("secret", func(g GroupAdder) {
		g.Command("add {name}", func(name string) error { return nil })
		g.Command("edit {name}", func(name string) error { return nil })
		g.Command("delete {name}", func(name string) error { return nil })
	})

	got := globalHelp("myapp")

	for _, want := range []string{
		"myapp 1.2.3",
		"Example app",
		"Usage:",
		"  myapp [arguments]",
		"  myapp {command} [arguments]",
		"Commands:",
		"  greet",
		"  secret",
		"    secret add",
		"    secret edit",
		"    secret delete",
		"  version",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("globalHelp missing %q in:\n%s", want, got)
		}
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

	Command("greet {name}", func(name string) error { return nil })

	got := globalHelp("myapp")
	if !strings.Contains(got, "  myapp {command} [arguments]") {
		t.Fatalf("globalHelp = %q", got)
	}
}
