package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunWith(t *testing.T) {
	originalCommands := app.commands
	app.commands = make(map[string]command)
	defer func() {
		app.commands = originalCommands
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
	app.commands = make(map[string]command)
	defer func() {
		app.commands = originalCommands
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
	app.commands = make(map[string]command)
	defer func() {
		app.commands = originalCommands
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

func TestGlobalHelp(t *testing.T) {
	originalName := app.name
	originalDescription := app.description
	originalVersion := app.version
	originalCommands := app.commands

	app.name = "myapp"
	app.description = "Example app"
	app.version = "1.2.3"
	app.commands = make(map[string]command)
	defer func() {
		app.name = originalName
		app.description = originalDescription
		app.version = originalVersion
		app.commands = originalCommands
	}()

	Command("{file}", func(file *string) error { return nil })
	Command("greet {name}", func(name string) error { return nil })
	Command("version", func() error { return nil })

	got := globalHelp("myapp")

	for _, want := range []string{
		"myapp 1.2.3",
		"Example app",
		"Usage:",
		"  myapp [arguments]",
		"  myapp {command} [arguments]",
		"Commands:",
		"  greet",
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
	originalName := app.name
	originalDescription := app.description
	originalVersion := app.version

	var out bytes.Buffer
	app.out = &out
	app.commands = make(map[string]command)
	app.name = "myapp"
	app.description = "Example app"
	app.version = "1.2.3"
	defer func() {
		app.out = originalOut
		app.commands = originalCommands
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
