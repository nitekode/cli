package cli

import (
	"bytes"
	"reflect"
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

func resetFlagsTestApp(t *testing.T) {
	t.Helper()

	originalCommands := app.commands
	originalGroups := app.groups
	originalFlags := app.flags
	app.commands = make(map[string]command)
	app.groups = make(map[string]*group)
	app.flags = nil

	t.Cleanup(func() {
		app.commands = originalCommands
		app.groups = originalGroups
		app.flags = originalFlags
	})
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

func TestParseFlagsBoolForms(t *testing.T) {
	set, err := compileFlagSet(testGlobalFlags{})
	if err != nil {
		t.Fatalf("compileFlagSet returned error: %v", err)
	}

	t.Run("implicit true", func(t *testing.T) {
		value, positionals, err := parseFlags(set, []commandArgument{{Name: "name", Kind: requiredArgument}}, []string{"--verbose", "alice"})
		if err != nil {
			t.Fatalf("parseFlags returned error: %v", err)
		}
		if !value.FieldByName("Verbose").Bool() {
			t.Fatalf("Verbose = false, want true")
		}
		if strings.Join(positionals, ",") != "alice" {
			t.Fatalf("positionals = %#v, want [alice]", positionals)
		}
	})

	t.Run("equals false", func(t *testing.T) {
		value, positionals, err := parseFlags(set, []commandArgument{{Name: "name", Kind: requiredArgument}}, []string{"--verbose=false", "alice"})
		if err != nil {
			t.Fatalf("parseFlags returned error: %v", err)
		}
		if value.FieldByName("Verbose").Bool() {
			t.Fatalf("Verbose = true, want false")
		}
		if strings.Join(positionals, ",") != "alice" {
			t.Fatalf("positionals = %#v, want [alice]", positionals)
		}
	})

	t.Run("space false remains positional", func(t *testing.T) {
		value, positionals, err := parseFlags(set, []commandArgument{{Name: "name", Kind: requiredArgument}, {Name: "other", Kind: requiredArgument}}, []string{"--verbose", "false", "alice"})
		if err != nil {
			t.Fatalf("parseFlags returned error: %v", err)
		}
		if !value.FieldByName("Verbose").Bool() {
			t.Fatalf("Verbose = false, want true")
		}
		if strings.Join(positionals, ",") != "false,alice" {
			t.Fatalf("positionals = %#v, want [false alice]", positionals)
		}
	})
}

func TestParseFlagsErrors(t *testing.T) {
	set, err := compileFlagSet(testCommandFlags{})
	if err != nil {
		t.Fatalf("compileFlagSet returned error: %v", err)
	}

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "unknown option",
			args:    []string{"--missing=value"},
			wantErr: `unknown option "missing"`,
		},
		{
			name:    "missing value for string",
			args:    []string{"--profile"},
			wantErr: `option "profile" expects a value`,
		},
		{
			name:    "invalid int value",
			args:    []string{"--base", "abc"},
			wantErr: "invalid value for option --base",
		},
		{
			name:    "invalid bool value",
			args:    []string{"--verbose=maybe"},
			wantErr: `flag "verbose" expects true or false`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := parseFlags(set, nil, tt.args)
			if err == nil {
				t.Fatalf("parseFlags returned nil error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("parseFlags error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
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

func TestGlobalFlagsCanBeIgnoredByHandler(t *testing.T) {
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
	var gotName string
	Command("greet {name}", "Greet someone.", func(name string) error {
		gotName = name
		return nil
	})

	if err := RunWith([]string{"test", "greet", "--verbose", "--profile", "prod", "alice"}); err != nil {
		t.Fatalf("RunWith returned error: %v", err)
	}

	if gotName != "alice" {
		t.Fatalf("name = %q, want %q", gotName, "alice")
	}
}

func TestGroupFlagsMustEmbedGlobalFlags(t *testing.T) {
	resetFlagsTestApp(t)

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

func TestRootCommandFlagsMustEmbedGlobalFlags(t *testing.T) {
	resetFlagsTestApp(t)

	type badCommandFlags struct {
		Round bool
	}

	GlobalFlags(testGlobalFlags{})

	defer func() {
		if recover() == nil {
			t.Fatal("Command did not panic")
		}
	}()

	Command("round", "Round a number.", func(flags badCommandFlags) error { return nil }, Flags(badCommandFlags{}))
}

func TestGroupedCommandFlagsMustEmbedGroupFlags(t *testing.T) {
	resetFlagsTestApp(t)

	type badCommandFlags struct {
		testGlobalFlags
		Round bool
	}

	GlobalFlags(testGlobalFlags{})

	defer func() {
		if recover() == nil {
			t.Fatal("Group did not panic")
		}
	}()

	Group("calc", "Calculator commands.", func(g GroupAdder) {
		g.Command("round", "Round a number.", func(flags badCommandFlags) error { return nil }, Flags(badCommandFlags{}))
	}, Flags(testGroupFlags{}))
}

func TestGroupedCommandFlagsMustEmbedGlobalFlagsWithoutGroupFlags(t *testing.T) {
	resetFlagsTestApp(t)

	type badCommandFlags struct {
		Round bool
	}

	GlobalFlags(testGlobalFlags{})

	defer func() {
		if recover() == nil {
			t.Fatal("Group did not panic")
		}
	}()

	Group("calc", "Calculator commands.", func(g GroupAdder) {
		g.Command("round", "Round a number.", func(flags badCommandFlags) error { return nil }, Flags(badCommandFlags{}))
	})
}

func TestHandlerFlagsMustMatchEffectiveFlags(t *testing.T) {
	resetFlagsTestApp(t)

	GlobalFlags(testGlobalFlags{})

	defer func() {
		if recover() == nil {
			t.Fatal("Group did not panic")
		}
	}()

	Group("calc", "Calculator commands.", func(g GroupAdder) {
		g.Command("base", "Show the base.", func(flags testGlobalFlags) error { return nil })
	}, Flags(testGroupFlags{}))
}

func TestFlagStorageKeepsOnlyLocalDeclarations(t *testing.T) {
	resetFlagsTestApp(t)

	GlobalFlags(testGlobalFlags{})
	Group("calc", "Calculator commands.", func(g GroupAdder) {
		g.Command("add", "Add two numbers.", func(flags testCommandFlags) error { return nil }, Flags(testCommandFlags{}))
	}, Flags(testGroupFlags{}))

	group := app.groups["calc"]
	cmd := group.commands["add"]

	if app.flags.typ != reflect.TypeOf(testGlobalFlags{}) {
		t.Fatalf("app.flags = %s, want testGlobalFlags", app.flags.typ)
	}
	if group.flags.typ != reflect.TypeOf(testGroupFlags{}) {
		t.Fatalf("group.flags = %s, want testGroupFlags", group.flags.typ)
	}
	if cmd.flags.typ != reflect.TypeOf(testCommandFlags{}) {
		t.Fatalf("cmd.flags = %s, want testCommandFlags", cmd.flags.typ)
	}
	if group.effectiveFlags().typ != reflect.TypeOf(testGroupFlags{}) {
		t.Fatalf("group.effectiveFlags = %s, want testGroupFlags", group.effectiveFlags().typ)
	}
	if cmd.effectiveFlags().typ != reflect.TypeOf(testCommandFlags{}) {
		t.Fatalf("cmd.effectiveFlags = %s, want testCommandFlags", cmd.effectiveFlags().typ)
	}
}

func TestEffectiveFlagsResolveFallbacks(t *testing.T) {
	resetFlagsTestApp(t)

	GlobalFlags(testGlobalFlags{})
	Command("root-global", "Use global flags.", func(flags testGlobalFlags) error { return nil })
	Command("root-local", "Use command flags.", func(flags testGroupFlags) error { return nil }, Flags(testGroupFlags{}))
	Group("calc", "Calculator commands.", func(g GroupAdder) {
		g.Command("group-local", "Use group flags.", func(flags testGroupFlags) error { return nil })
		g.Command("command-local", "Use command flags.", func(flags testCommandFlags) error { return nil }, Flags(testCommandFlags{}))
	}, Flags(testGroupFlags{}))
	Group("plain", "Plain commands.", func(g GroupAdder) {
		g.Command("global", "Use global flags.", func(flags testGlobalFlags) error { return nil })
	})

	if got := app.commands["root-global"].effectiveFlags().typ; got != reflect.TypeOf(testGlobalFlags{}) {
		t.Fatalf("root global effective flags = %s, want testGlobalFlags", got)
	}
	if got := app.commands["root-local"].effectiveFlags().typ; got != reflect.TypeOf(testGroupFlags{}) {
		t.Fatalf("root local effective flags = %s, want testGroupFlags", got)
	}
	if got := app.groups["calc"].commands["group-local"].effectiveFlags().typ; got != reflect.TypeOf(testGroupFlags{}) {
		t.Fatalf("group local effective flags = %s, want testGroupFlags", got)
	}
	if got := app.groups["calc"].commands["command-local"].effectiveFlags().typ; got != reflect.TypeOf(testCommandFlags{}) {
		t.Fatalf("command local effective flags = %s, want testCommandFlags", got)
	}
	if got := app.groups["plain"].commands["global"].effectiveFlags().typ; got != reflect.TypeOf(testGlobalFlags{}) {
		t.Fatalf("group global fallback effective flags = %s, want testGlobalFlags", got)
	}
}

func TestGroupHelpShowsInheritedGlobalOptions(t *testing.T) {
	resetFlagsTestApp(t)

	GlobalFlags(testGlobalFlags{})
	Group("calc", "Calculator commands.", func(g GroupAdder) {
		g.Command("add", "Add two numbers.", func(flags testGlobalFlags) error { return nil })
	})

	got := groupHelp("myapp", app.groups["calc"])
	for _, want := range []string{
		"  myapp calc {command} [options] [arguments]",
		"\nOptions:\n",
		"  --verbose",
		"  --profile[=PROFILE]",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("groupHelp missing %q in:\n%s", want, got)
		}
	}
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
