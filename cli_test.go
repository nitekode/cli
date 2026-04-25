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
}
