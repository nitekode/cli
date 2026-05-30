package cli

import (
	"strings"
	"testing"
)

func withCleanRegistry(t *testing.T) {
	t.Helper()
	originalCommands := app.commands
	originalGroups := app.groups
	originalLabels := app.labels
	app.commands = make(map[string]command)
	app.groups = make(map[string]*group)
	app.labels = nil
	t.Cleanup(func() {
		app.commands = originalCommands
		app.groups = originalGroups
		app.labels = originalLabels
	})
}

func TestGlobalHelpRendersLabelSections(t *testing.T) {
	withCleanRegistry(t)

	Command("status", "Show status.", func() error { return nil })
	Label("Stack", func(l LabelAdder) {
		l.Command("build {env}", "Build the stack.", func(env string) error { return nil })
		l.Command("start", "Start the stack.", func() error { return nil })
	})
	Label("Run", func(l LabelAdder) {
		l.Command("artisan {cmd}", "Run an artisan command.", func(cmd string) error { return nil })
	})

	got := globalHelp("lstack")

	for _, want := range []string{
		"Commands:",
		"  status   Show status.",
		"Stack:",
		"  build  Build the stack.",
		"  start  Start the stack.",
		"Run:",
		"  artisan  Run an artisan command.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("globalHelp missing %q in:\n%s", want, got)
		}
	}

	// Labeled commands belong only to their section, not the flat Commands list.
	flat := got[strings.Index(got, "Commands:"):strings.Index(got, "Stack:")]
	for _, leaked := range []string{"build", "start", "artisan"} {
		if strings.Contains(flat, leaked) {
			t.Fatalf("labeled command %q leaked into flat Commands block:\n%s", leaked, flat)
		}
	}

	// Headings keep declaration order: Stack before Run.
	if strings.Index(got, "Stack:") > strings.Index(got, "Run:") {
		t.Fatalf("labels rendered out of declaration order:\n%s", got)
	}
}

func TestLabeledCommandInvokesWithoutPrefix(t *testing.T) {
	withCleanRegistry(t)

	var gotEnv string
	Label("Stack", func(l LabelAdder) {
		l.Command("build {env}", "Build the stack.", func(env string) error {
			gotEnv = env
			return nil
		})
	})

	if err := RunWith([]string{"lstack", "build", "prod"}); err != nil {
		t.Fatalf("RunWith returned error: %v", err)
	}
	if gotEnv != "prod" {
		t.Fatalf("gotEnv = %q, want %q", gotEnv, "prod")
	}
}

func TestLabelOmitsHiddenCommandAndEmptySection(t *testing.T) {
	withCleanRegistry(t)

	Label("Stack", func(l LabelAdder) {
		l.Command("build", "Build the stack.", func() error { return nil })
		l.Command("debug", "Internal.", func() error { return nil }, Hidden())
	})
	Label("Empty", func(l LabelAdder) {
		l.Command("secret", "Hidden only.", func() error { return nil }, Hidden())
	})

	got := globalHelp("lstack")

	if !strings.Contains(got, "Stack:") || !strings.Contains(got, "  build") {
		t.Fatalf("expected Stack section with build:\n%s", got)
	}
	if strings.Contains(got, "debug") {
		t.Fatalf("hidden labeled command should not render:\n%s", got)
	}
	if strings.Contains(got, "Empty:") {
		t.Fatalf("label with no visible commands should be omitted:\n%s", got)
	}
}

func TestLabelDuplicateTitlePanics(t *testing.T) {
	withCleanRegistry(t)
	defer func() {
		r := recover()
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "duplicate label") {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()

	Label("Stack", func(l LabelAdder) {})
	Label("Stack", func(l LabelAdder) {})
}

func TestLabelCommandConflictWithExistingCommandPanics(t *testing.T) {
	withCleanRegistry(t)
	defer func() {
		r := recover()
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "duplicate command") {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()

	Command("build", "Build.", func() error { return nil })
	Label("Stack", func(l LabelAdder) {
		l.Command("build", "Build the stack.", func() error { return nil })
	})
}
