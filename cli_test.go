package cli

import (
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
