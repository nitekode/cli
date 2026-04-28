package cli

import (
	"bytes"
	"errors"
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

	Group("calc", func(g GroupAdder) {
		g.Command("add {a} {b}", func(a string, b string) error { return nil })
	})

	if err := RunWith([]string{"myapp", "help", "calc"}); err != nil {
		t.Fatalf("RunWith returned error: %v", err)
	}

	for _, want := range []string{
		"Usage:",
		"  myapp {command} [arguments]",
		`Commands in the "calc" group:`,
		"  calc add",
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

	Command("greet {name}", func(name string) error { return nil })

	if err := RunWith([]string{"myapp", "help", "greet"}); err != nil {
		t.Fatalf("RunWith returned error: %v", err)
	}

	if !strings.Contains(out.String(), "  myapp greet <name>\n") {
		t.Fatalf("help output = %q", out.String())
	}
	if !strings.Contains(out.String(), "\nArguments:\n  name\n") {
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

	Group("calc", func(g GroupAdder) {
		g.Command("add {a} {b}", func(a string, b string) error { return nil })
	})

	if err := RunWith([]string{"myapp", "help", "calc", "add"}); err != nil {
		t.Fatalf("RunWith returned error: %v", err)
	}

	if !strings.Contains(out.String(), "  myapp calc add <a> <b>\n") {
		t.Fatalf("help output = %q", out.String())
	}
	for _, want := range []string{
		"\nArguments:\n",
		"  a\n",
		"  b\n",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("help output missing %q in:\n%s", want, out.String())
		}
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
		"  help",
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

	Command("version", func() error {
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

func TestGlobalHelpOmitsHiddenCommandAndGroup(t *testing.T) {
	originalCommands := app.commands
	originalGroups := app.groups
	app.commands = make(map[string]command)
	app.groups = make(map[string]*group)
	defer func() {
		app.commands = originalCommands
		app.groups = originalGroups
	}()

	Command("visible", func() error { return nil })
	Command("hidden", func() error { return nil }, Hidden())
	Group("secret", func(g GroupAdder) {
		g.Command("add", func() error { return nil })
	}, Hidden())

	got := globalHelp("myapp")

	if strings.Contains(got, "\n  hidden\n") {
		t.Fatalf("globalHelp should omit hidden command:\n%s", got)
	}
	if strings.Contains(got, "\n  secret\n") || strings.Contains(got, "secret add") {
		t.Fatalf("globalHelp should omit hidden group:\n%s", got)
	}
	if !strings.Contains(got, "\n  visible\n") {
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

	Command("greet {name}", func(name string) error { return nil })

	got := globalHelp("myapp")
	if !strings.Contains(got, "  myapp {command} [arguments]") {
		t.Fatalf("globalHelp = %q", got)
	}
}
