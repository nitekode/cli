package cli

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestCompileCommandArguments(t *testing.T) {
	tests := []struct {
		name    string
		sig     string
		handler any
		wantErr string
	}{
		{
			name:    "matches required optional defaulted and repeated arguments",
			sig:     "greet {name} {title=friend} {suffix} {others}",
			handler: func(string, string, *string, ...string) error { return nil },
		},
		{
			name:    "not a function",
			sig:     "greet {name}",
			handler: "nope",
			wantErr: "handler must be a function",
		},
		{
			name:    "too few parameters",
			sig:     "greet {name} {title}",
			handler: func(string) error { return nil },
			wantErr: "expects 1 parameters, signature defines 2 arguments",
		},
		{
			name:    "required argument must be string",
			sig:     "greet {name}",
			handler: func(int) error { return nil },
			wantErr: "must map to string, *string, or ...string",
		},
		{
			name:    "pointer argument cannot have default",
			sig:     "greet {name=friend}",
			handler: func(*string) error { return nil },
			wantErr: "cannot have a default because it maps to a *string parameter",
		},
		{
			name:    "repeated argument cannot have default",
			sig:     "greet {names=friend}",
			handler: func(...string) error { return nil },
			wantErr: "cannot have a default because it is repeated",
		},
		{
			name:    "variadic argument must be string slice",
			sig:     "greet {names}",
			handler: func(...int) error { return nil },
			wantErr: "must map to a ...string parameter",
		},
		{
			name:    "missing return value",
			sig:     "greet {name}",
			handler: func(string) {},
			wantErr: "must return a single error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sig, err := parseSignature(tt.sig)
			if err != nil {
				t.Fatalf("parseSignature returned error: %v", err)
			}

			handlerValue := reflect.ValueOf(tt.handler)
			if tt.wantErr == "handler must be a function" {
				handlerValue = reflect.Value{}
			}

			var handlerType reflect.Type
			if handlerValue.IsValid() {
				handlerType = handlerValue.Type()
			}

			_, _, err = compileCommandArguments(sig, handlerType)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("compileCommandArguments returned error: %v", err)
				}

				return
			}

			if err == nil {
				t.Fatalf("compileCommandArguments returned nil error, want %q", tt.wantErr)
			}

			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("compileCommandArguments error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestInvokeCommand(t *testing.T) {
	t.Run("applies defaults optionals and repeated values", func(t *testing.T) {
		var gotName string
		var gotTitle string
		var gotSuffix *string
		var gotOthers []string

		cmd := command{
			name: "greet",
			handler: reflect.ValueOf(func(name string, title string, suffix *string, others ...string) error {
				gotName = name
				gotTitle = title
				gotSuffix = suffix
				gotOthers = append([]string(nil), others...)
				return nil
			}),
			handlerType: reflect.TypeOf(func(string, string, *string, ...string) error { return nil }),
			arguments: []commandArgument{
				{Name: "name", Kind: requiredArgument},
				{Name: "title", Kind: defaultArgument, Default: "friend"},
				{Name: "suffix", Kind: optionalArgument},
				{Name: "others", Kind: repeatedArgument},
			},
		}

		if err := cmd.invoke([]string{"alice"}); err != nil {
			t.Fatalf("invoke returned error: %v", err)
		}

		if gotName != "alice" {
			t.Fatalf("gotName = %q, want %q", gotName, "alice")
		}
		if gotTitle != "friend" {
			t.Fatalf("gotTitle = %q, want %q", gotTitle, "friend")
		}
		if gotSuffix != nil {
			t.Fatalf("gotSuffix = %v, want nil", *gotSuffix)
		}
		if len(gotOthers) != 0 {
			t.Fatalf("gotOthers = %#v, want empty", gotOthers)
		}

		if err := cmd.invoke([]string{"alice", "captain", "jr", "bob", "carol"}); err != nil {
			t.Fatalf("invoke returned error: %v", err)
		}

		if gotTitle != "captain" {
			t.Fatalf("gotTitle = %q, want %q", gotTitle, "captain")
		}
		if gotSuffix == nil || *gotSuffix != "jr" {
			t.Fatalf("gotSuffix = %v, want %q", gotSuffix, "jr")
		}
		if strings.Join(gotOthers, ",") != "bob,carol" {
			t.Fatalf("gotOthers = %#v, want [bob carol]", gotOthers)
		}
	})

	t.Run("rejects missing required argument", func(t *testing.T) {
		cmd := command{
			name:        "greet",
			handler:     reflect.ValueOf(func(string) error { return nil }),
			handlerType: reflect.TypeOf(func(string) error { return nil }),
			arguments: []commandArgument{
				{Name: "name", Kind: requiredArgument},
			},
		}

		err := cmd.invoke(nil)
		if err == nil || !strings.Contains(err.Error(), "missing arguments: got 0, want at least 1") {
			t.Fatalf("invoke error = %v", err)
		}
	})

	t.Run("returns handler error", func(t *testing.T) {
		cmd := command{
			name:        "greet",
			handler:     reflect.ValueOf(func(string) error { return errors.New("boom") }),
			handlerType: reflect.TypeOf(func(string) error { return errors.New("boom") }),
			arguments: []commandArgument{
				{Name: "name", Kind: requiredArgument},
			},
		}

		err := cmd.invoke([]string{"alice"})
		if err == nil || !strings.Contains(err.Error(), "boom") {
			t.Fatalf("invoke error = %v", err)
		}
	})
}

func TestCommandUsage(t *testing.T) {
	cmd := command{
		name: "greet",
		arguments: []commandArgument{
			{Name: "name", Kind: requiredArgument},
			{Name: "title", Kind: defaultArgument, Default: "friend"},
			{Name: "suffix", Kind: optionalArgument},
			{Name: "others", Kind: repeatedArgument},
		},
	}

	got := cmd.usage("myapp")
	want := "myapp greet <name> [title] [suffix] [others...]"
	if got != want {
		t.Fatalf("usage = %q, want %q", got, want)
	}
}

func TestInvokeCommandWithMiddleware(t *testing.T) {
	originalMiddleware := app.middleware
	app.middleware = nil
	defer func() {
		app.middleware = originalMiddleware
	}()

	var calls []string

	Use(
		func(next func() error) error {
			calls = append(calls, "mw1-before")
			err := next()
			calls = append(calls, "mw1-after")
			return err
		},
		func(next func() error) error {
			calls = append(calls, "mw2-before")
			err := next()
			calls = append(calls, "mw2-after")
			return err
		},
	)

	cmd := command{
		name: "greet",
		handler: reflect.ValueOf(func() error {
			calls = append(calls, "handler")
			return nil
		}),
		handlerType: reflect.TypeOf(func() error { return nil }),
	}

	if err := cmd.invoke(nil, app.middleware...); err != nil {
		t.Fatalf("invoke returned error: %v", err)
	}

	want := []string{"mw1-before", "mw2-before", "handler", "mw2-after", "mw1-after"}
	if !slices.Equal(calls, want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
}

func TestInvokeCommandMiddlewareCanAbort(t *testing.T) {
	originalMiddleware := app.middleware
	app.middleware = nil
	defer func() {
		app.middleware = originalMiddleware
	}()

	calledHandler := false
	Use(func(next func() error) error {
		return errors.New("blocked")
	})

	cmd := command{
		name: "greet",
		handler: reflect.ValueOf(func() error {
			calledHandler = true
			return nil
		}),
		handlerType: reflect.TypeOf(func() error { return nil }),
	}

	err := cmd.invoke(nil, app.middleware...)
	if err == nil || !strings.Contains(err.Error(), "blocked") {
		t.Fatalf("invoke error = %v", err)
	}
	if calledHandler {
		t.Fatalf("handler should not have been called")
	}
}

func TestInvokeCommandWithScopedMiddlewareOrder(t *testing.T) {
	originalMiddleware := app.middleware
	app.middleware = nil
	defer func() {
		app.middleware = originalMiddleware
	}()

	var calls []string

	Use(func(next func() error) error {
		calls = append(calls, "global-before")
		err := next()
		calls = append(calls, "global-after")
		return err
	})

	groupMiddleware := []MiddlewareFunc{
		func(next func() error) error {
			calls = append(calls, "group-before")
			err := next()
			calls = append(calls, "group-after")
			return err
		},
	}

	commandMiddleware := []MiddlewareFunc{
		func(next func() error) error {
			calls = append(calls, "command-before")
			err := next()
			calls = append(calls, "command-after")
			return err
		},
	}

	cmd := command{
		name: "add",
		handler: reflect.ValueOf(func() error {
			calls = append(calls, "handler")
			return nil
		}),
		handlerType: reflect.TypeOf(func() error { return nil }),
		middleware:  commandMiddleware,
	}

	all := append([]MiddlewareFunc(nil), app.middleware...)
	all = append(all, groupMiddleware...)
	all = append(all, cmd.middleware...)

	if err := cmd.invoke(nil, all...); err != nil {
		t.Fatalf("invoke returned error: %v", err)
	}

	want := []string{
		"global-before",
		"group-before",
		"command-before",
		"handler",
		"command-after",
		"group-after",
		"global-after",
	}
	if !slices.Equal(calls, want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
}
