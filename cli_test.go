package cli

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestRunWith(t *testing.T) {
	originalCommands := app.commands
	originalGroups := app.groups
	originalName := app.name
	originalVersion := app.version
	originalCommit := app.commit
	originalBuiltAt := app.builtAt
	app.commands = make(map[string]command)
	app.groups = make(map[string]*group)
	app.name = "myapp"
	app.version = "1.2.3"
	app.commit = "unknown"
	app.builtAt = "unknown"
	defer func() {
		app.commands = originalCommands
		app.groups = originalGroups
		app.name = originalName
		app.version = originalVersion
		app.commit = originalCommit
		app.builtAt = originalBuiltAt
	}()

	Command("greet {name} {title=friend} {suffix} {others}", "Greet someone.", func(name string, title string, suffix *string, others ...string) error {
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

	var out bytes.Buffer
	prevOut := app.out
	app.out = &out
	if err := RunWith([]string{"test", "version"}); err != nil {
		t.Fatalf("RunWith version returned error: %v", err)
	}
	app.out = prevOut
	if out.String() != "1.2.3\n" {
		t.Fatalf("version output = %q, want %q", out.String(), "1.2.3\n")
	}
	out.Reset()
	app.out = &out
	if err := RunWith([]string{"test", "help"}); err != nil {
		t.Fatalf("RunWith help returned error: %v", err)
	}
	app.out = prevOut
	if !strings.Contains(out.String(), "Commands:\n") {
		t.Fatalf("help output = %q", out.String())
	}

	app.commands = make(map[string]command)
	app.groups = make(map[string]*group)

	var gotPath *string
	Command("{path}", "Read a path.", func(path *string) error {
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
	Command("{file}", "Read a file.", func(file *string) error {
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

	Command("{value}", "Handle a value.", func(value string) error {
		called = "root:" + value
		return nil
	})

	Command("greet {name}", "Greet someone.", func(name string) error {
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

	Group("secret", "Secret commands.", func(g GroupAdder) {
		g.Command("add {name}", "Add a secret.", func(name string) error {
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

	Command("{value}", "Handle a value.", func(value string) error {
		called = "root:" + value
		return nil
	})

	Group("secret", "Secret commands.", func(g GroupAdder) {
		g.Command("add", "Add a secret.", func() error {
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

func TestExplicitVersionCommandOverridesBuiltIn(t *testing.T) {
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

	Command("version", "Show custom version.", func() error {
		_, err := fmt.Fprintln(app.out, "custom version")
		return err
	})

	if err := RunWith([]string{"test", "version"}); err != nil {
		t.Fatalf("RunWith returned error: %v", err)
	}

	if out.String() != "custom version\n" {
		t.Fatalf("version output = %q, want %q", out.String(), "custom version\n")
	}
}

func TestVersionStringIncludesOptionalBuildMetadata(t *testing.T) {
	originalVersion := app.version
	originalCommit := app.commit
	originalBuiltAt := app.builtAt
	defer func() {
		app.version = originalVersion
		app.commit = originalCommit
		app.builtAt = originalBuiltAt
	}()

	app.version = "v1.2.3"
	app.builtAt = "2024-06-24 22:11:35"
	app.commit = "abcd123"

	if got := versionString(); got != "v1.2.3 (2024-06-24 22:11:35) [abcd123]" {
		t.Fatalf("versionString = %q", got)
	}

	app.builtAt = "unknown"
	app.commit = "unknown"

	if got := versionString(); got != "v1.2.3" {
		t.Fatalf("versionString = %q", got)
	}
}
