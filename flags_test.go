package cli

import (
	"bytes"
	"strings"
	"testing"
)

type testGlobalFlags struct {
	Verbose bool   `desc:"verbose output"`
	Profile string `default:"dev" desc:"profile name"`
}

type testGroupFlags struct {
	testGlobalFlags
	Base int `default:"10" desc:"number base"`
}

type testCommandFlags struct {
	testGroupFlags
	Round bool `desc:"round the result"`
}

func TestRunWithPopulatesGlobalFlags(t *testing.T) {
	originalCommands := app.commands
	originalGroups := app.groups
	originalFlags := app.flags
	app.commands = make(map[string]command)
	app.groups = make(map[string]*group)
	app.flags = nil
	defer func() {
		app.commands = originalCommands
		app.groups = originalGroups
		app.flags = originalFlags
	}()

	var gotFlags testGlobalFlags
	var gotName string

	GlobalFlags(testGlobalFlags{})
	Command("greet {name}", "Greet someone.", func(flags testGlobalFlags, name string) error {
		gotFlags = flags
		gotName = name
		return nil
	})

	if err := RunWith([]string{"test", "greet", "--verbose", "--profile", "prod", "alice"}); err != nil {
		t.Fatalf("RunWith returned error: %v", err)
	}

	if !gotFlags.Verbose {
		t.Fatalf("Verbose = false, want true")
	}
	if gotFlags.Profile != "prod" {
		t.Fatalf("Profile = %q, want %q", gotFlags.Profile, "prod")
	}
	if gotName != "alice" {
		t.Fatalf("name = %q, want %q", gotName, "alice")
	}
}

func TestRunWithPopulatesComposedFlagsForGroupedCommand(t *testing.T) {
	originalCommands := app.commands
	originalGroups := app.groups
	originalFlags := app.flags
	app.commands = make(map[string]command)
	app.groups = make(map[string]*group)
	app.flags = nil
	defer func() {
		app.commands = originalCommands
		app.groups = originalGroups
		app.flags = originalFlags
	}()

	var gotFlags testCommandFlags
	var gotA string
	var gotB string

	GlobalFlags(testGlobalFlags{})
	Group("calc", "Calculator commands.", func(g GroupAdder) {
		g.Command("add {a} {b}", "Add two numbers.", func(flags testCommandFlags, a string, b string) error {
			gotFlags = flags
			gotA = a
			gotB = b
			return nil
		}, Flags(testCommandFlags{}))
	}, Flags(testGroupFlags{}))

	if err := RunWith([]string{"test", "calc", "add", "--verbose", "--base", "16", "--round", "a", "b"}); err != nil {
		t.Fatalf("RunWith returned error: %v", err)
	}

	if !gotFlags.Verbose {
		t.Fatalf("Verbose = false, want true")
	}
	if gotFlags.Profile != "dev" {
		t.Fatalf("Profile = %q, want %q", gotFlags.Profile, "dev")
	}
	if gotFlags.Base != 16 {
		t.Fatalf("Base = %d, want 16", gotFlags.Base)
	}
	if !gotFlags.Round {
		t.Fatalf("Round = false, want true")
	}
	if gotA != "a" || gotB != "b" {
		t.Fatalf("args = %q, %q, want a, b", gotA, gotB)
	}
}

func TestGlobalFlagsRequireHandlerParameter(t *testing.T) {
	originalCommands := app.commands
	originalGroups := app.groups
	originalFlags := app.flags
	app.commands = make(map[string]command)
	app.groups = make(map[string]*group)
	app.flags = nil
	defer func() {
		app.commands = originalCommands
		app.groups = originalGroups
		app.flags = originalFlags
	}()

	GlobalFlags(testGlobalFlags{})

	defer func() {
		if recover() == nil {
			t.Fatal("Command did not panic")
		}
	}()

	Command("greet {name}", "Greet someone.", func(name string) error { return nil })
}

func TestGroupFlagsMustEmbedGlobalFlags(t *testing.T) {
	originalCommands := app.commands
	originalGroups := app.groups
	originalFlags := app.flags
	app.commands = make(map[string]command)
	app.groups = make(map[string]*group)
	app.flags = nil
	defer func() {
		app.commands = originalCommands
		app.groups = originalGroups
		app.flags = originalFlags
	}()

	type badGroupFlags struct {
		Base int
	}

	GlobalFlags(testGlobalFlags{})

	defer func() {
		if recover() == nil {
			t.Fatal("Group did not panic")
		}
	}()

	Group("calc", "Calculator commands.", func(g GroupAdder) {}, Flags(badGroupFlags{}))
}

func TestCommandHelpShowsOptions(t *testing.T) {
	originalCommands := app.commands
	originalGroups := app.groups
	originalFlags := app.flags
	originalOut := app.out
	app.commands = make(map[string]command)
	app.groups = make(map[string]*group)
	app.flags = nil
	var out bytes.Buffer
	app.out = &out
	defer func() {
		app.commands = originalCommands
		app.groups = originalGroups
		app.flags = originalFlags
		app.out = originalOut
	}()

	GlobalFlags(testGlobalFlags{})
	Command("greet {name}", "Greet someone.", func(flags testGlobalFlags, name string) error { return nil })

	if err := RunWith([]string{"myapp", "help", "greet"}); err != nil {
		t.Fatalf("RunWith returned error: %v", err)
	}

	for _, want := range []string{
		"  myapp greet [options] <name>\n",
		"\nArguments:\n  name\n",
		"\nOptions:\n",
		"  --verbose",
		"  --profile[=PROFILE]",
		"(default=dev)",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("help output missing %q in:\n%s", want, out.String())
		}
	}
	if strings.Index(out.String(), "\nArguments:\n") > strings.Index(out.String(), "\nOptions:\n") {
		t.Fatalf("arguments should be listed before options:\n%s", out.String())
	}
}

func TestGlobalHelpShowsOptions(t *testing.T) {
	originalCommands := app.commands
	originalGroups := app.groups
	originalFlags := app.flags
	originalDescription := app.description
	app.commands = make(map[string]command)
	app.groups = make(map[string]*group)
	app.flags = nil
	app.description = "Example app"
	defer func() {
		app.commands = originalCommands
		app.groups = originalGroups
		app.flags = originalFlags
		app.description = originalDescription
	}()

	GlobalFlags(testGlobalFlags{})
	Command("greet {name}", "Greet someone.", func(flags testGlobalFlags, name string) error { return nil })

	got := globalHelp("myapp")
	for _, want := range []string{
		"  myapp {command} [options] [arguments]",
		"\nOptions:\n",
		"  --verbose",
		"  --profile[=PROFILE]",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("globalHelp missing %q in:\n%s", want, got)
		}
	}
}
