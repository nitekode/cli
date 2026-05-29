# cli

`github.com/nitekode/cli` is a small Go library for building command-line applications with functions as command handlers. Commands are registered in Go code, arguments are described in a compact signature string, and flags are defined with structs.

## Install

```sh
go get github.com/nitekode/cli
```

## Usage

```go
package main

import (
	"fmt"

	"github.com/nitekode/cli"
)

func main() {
	cli.Name("hello")
	cli.Version("1.0.0")

	cli.Command("greet {name}", "Greet someone.", func(name string) error {
		fmt.Fprintf(cli.Out(), "Hello, %s\n", name)
		return nil
	})

	cli.Run()
}
```

Run it:

```sh
go run . greet Alice
go run . help
go run . help greet
go run . version
```

## Commands

Register a command with `Command(signature, description, handler, opts...)`.

```go
cli.Command("copy {src} {dst}", "Copy a file.", func(src string, dst string) error {
	return copyFile(src, dst)
})
```

Handlers must be functions that return a single `error`. Positional arguments are bound from the command signature to handler parameters.

Supported positional parameter forms:

```go
cli.Command("required {name}", "Required argument.", func(name string) error {
	return nil
})

cli.Command("optional {name}", "Optional argument.", func(name *string) error {
	return nil
})

cli.Command("default {name=world}", "Default argument.", func(name string) error {
	return nil
})

cli.Command("many {items}", "Repeated arguments.", func(items ...string) error {
	return nil
})
```

Argument descriptions can be written in the signature or supplied with `ArgDesc`:

```go
cli.Command(
	"greet {name:person to greet}",
	"Greet someone.",
	func(name string) error { return nil },
)

cli.Command(
	"sleep {duration=1}",
	"Sleep for a duration.",
	func(duration string) error { return nil },
	cli.ArgDesc("duration", "sleep time in seconds"),
)
```

## Flags

Flags are declared with structs. A handler can receive the applicable flags struct as its first parameter.

```go
type globalFlags struct {
	Verbose bool   `flag:"verbose,v" desc:"print verbose output"`
	Profile string `flag:"profile" default:"dev" desc:"profile name"`
}

func main() {
	cli.GlobalFlags[globalFlags]()

	cli.Command("deploy {service}", "Deploy a service.", func(flags globalFlags, service string) error {
		if flags.Verbose {
			fmt.Fprintf(cli.Out(), "deploying %s with profile %s\n", service, flags.Profile)
		}
		return nil
	})

	cli.Run()
}
```

Examples:

```sh
app deploy api --verbose
app deploy api -v --profile prod
app deploy api --profile=prod
```

Supported flag field types are `bool`, `string`, and `int`.

Supported tags:

- `flag:"long[,short]"` sets the required long option name and optional one-character short option.
- `default:"value"` sets a default value.
- `desc:"text"` adds help text.

Every exported field in a registered flags struct must declare a `flag` tag.

Boolean flags can be passed without a value, or with an explicit value:

```sh
app deploy api --verbose
app deploy api --verbose=false
```

## Groups

Use `Group` to organize related commands.

```go
cli.Group("calc", "Calculator commands.", func(g cli.GroupAdder) {
	g.Command("add {a} {b}", "Add two numbers.", func(a string, b string) error {
		return nil
	})
	g.Command("sub {a} {b}", "Subtract two numbers.", func(a string, b string) error {
		return nil
	})
})
```

Run grouped commands as:

```sh
app calc add 1 2
app help calc
app help calc add
```

## Scoped Flags

Flags can be declared globally, on a group, or on a command. More specific flag structs embed the parent scope's flag struct.

```go
type globalFlags struct {
	Verbose bool `flag:"verbose,v"`
}

type formatFlags struct {
	globalFlags
	Prefix string `flag:"prefix,p"`
}

type upperFlags struct {
	formatFlags
	Strong bool `flag:"strong,s"`
}

func main() {
	cli.GlobalFlags[globalFlags]()

	cli.Group("format", "Format text.", func(g cli.GroupAdder) {
		g.Command("upper {text}", "Uppercase text.", upperHandler, cli.Flags[upperFlags]())
	}, cli.Flags[formatFlags]())

	cli.Run()
}

func upperHandler(flags upperFlags, text string) error {
	if flags.Strong {
		text += "!"
	}
	fmt.Fprintf(cli.Out(), "%s%s\n", flags.Prefix, text)
	return nil
}
```

The handler may omit the flags parameter. Flags are still parsed and validated, but not passed to the handler.

## Middleware

Middleware wraps command execution. It can be registered globally with `Use`, on a group with `Middleware`, or on a command with `Middleware`.

```go
func logRun(next func() error) error {
	fmt.Fprintln(cli.Err(), "starting")
	err := next()
	fmt.Fprintln(cli.Err(), "done")
	return err
}

func main() {
	cli.Use(logRun)

	cli.Command("work", "Do work.", func() error {
		fmt.Fprintln(cli.Out(), "working")
		return nil
	})

	cli.Run()
}
```

Middleware can return an error without calling `next` to stop execution.

## Help and Version

`help` and `version` commands are added automatically unless you register commands with those names.

```go
cli.Name("myapp")
cli.Description("Example application.")
cli.Version("1.2.3")
cli.Build("1.2.3", "abc123", "2026-05-07")
```

Useful commands:

```sh
app help
app help COMMAND
app help GROUP
app help GROUP COMMAND
app version
```

## Input and Output

Use `In`, `Out`, and `Err` inside handlers instead of accessing `os.Stdin`, `os.Stdout`, and `os.Stderr` directly. This keeps commands easier to test.

```go
func handler() error {
	_, err := io.Copy(cli.Out(), cli.In())
	return err
}
```

For tests or temporary redirection:

```go
var out bytes.Buffer
previous := cli.SetOut(&out)
defer cli.SetOut(previous)
```

## Testing Commands

Use `RunWith` to execute a CLI with explicit arguments.

```go
func TestGreet(t *testing.T) {
	var out bytes.Buffer
	previous := cli.SetOut(&out)
	defer cli.SetOut(previous)

	cli.Command("greet {name}", "Greet someone.", func(name string) error {
		fmt.Fprintf(cli.Out(), "Hello, %s\n", name)
		return nil
	})

	if err := cli.RunWith([]string{"app", "greet", "Alice"}); err != nil {
		t.Fatal(err)
	}

	if out.String() != "Hello, Alice\n" {
		t.Fatalf("unexpected output: %q", out.String())
	}
}
```

The package keeps global registration state, so tests that register commands should isolate or reset that state as appropriate.

## Examples

Example programs are in [`examples/`](examples/):

- [`examples/cat`](examples/cat) shows a root command and input/output helpers.
- [`examples/math`](examples/math) shows named commands and groups.
- [`examples/text`](examples/text) shows scoped and composed flags.
- [`examples/sleep`](examples/sleep) shows middleware.

## Development

Run the test suite:

```sh
go test ./...
```

## License

MIT. See [LICENSE](LICENSE).
