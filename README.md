# cli

**Build Go command-line apps where your functions *are* the commands, with no flag parsing or struct-of-structs ceremony.**

```go
cli.Command("greet {name}", "Greet someone", func(name string) error {
	fmt.Fprintf(cli.Out(), "Hello, %s\n", name)
	return nil
})
cli.Run()
```

```sh
$ greet Alice
Hello, Alice
```

The signature `"greet {name}"` says there's one argument; the handler takes one `string`. The library wires them together, validates the input, and calls your function. That's the whole model.

## Features

- **Functions are handlers.** Register any `func(...) error`. Positional arguments bind straight to parameters by position, with types that mean something: `string` is required, `*string` is optional, `...string` soaks up the rest.
- **Flags are structs.** Declare a struct with `flag:` tags and your handler receives it filled in. Bool, string, int, and repeatable count flags (`-vvv`) all just work.
- **Scopes that compose by embedding.** Global, group, and command flags share state through plain struct embedding. No registration graph to reason about; the type *is* the relationship.
- **Groups, middleware, and auto-generated help.** Organize subcommands, wrap execution with `func(next) error`, and get `help` and `version` for free.
- **Testable I/O.** `In()`, `Out()`, and `Err()` are swappable, so commands are easy to drive from a test with `RunWith`.

## Install

```sh
go get github.com/nitekode/cli
```

## Quick Start

```go
package main

import (
	"fmt"

	"github.com/nitekode/cli"
)

type flags struct {
	Verbose bool   `flag:"verbose,v" desc:"print extra detail"`
	Profile string `flag:"profile,p" default:"dev" desc:"profile to deploy with"`
}

func main() {
	cli.Name("deployer")
	cli.Version("1.0.0")
	cli.GlobalFlags[flags]()

	cli.Command("deploy {service}", "Deploy a service", func(f flags, service string) error {
		if f.Verbose {
			fmt.Fprintf(cli.Out(), "using profile %s\n", f.Profile)
		}
		fmt.Fprintf(cli.Out(), "deployed %s\n", service)
		return nil
	})

	cli.Run()
}
```

```sh
$ deployer deploy api -v --profile prod
using profile prod
deployed api

$ deployer help deploy
$ deployer version
```

## Why This Library?

Most Go CLI libraries make you describe your command twice: once as configuration (a `cobra.Command` value, a slice of `cli.Flag` interfaces) and again as the function that does the work, with a `Run` field or `c.String("name")` lookups bridging the two. The config and the code drift apart, and you spend your time keeping them in sync.

Here the function is the source of truth. The signature string names the arguments, the parameter types decide whether they're required or optional, and the flags struct is the thing your handler actually reads. There's no `args[0]`, no `ctx.String("profile")`, no untyped lookups, just parameters and fields the compiler already checks for you.

It's deliberately small. If you want a kitchen sink, this isn't it. If you want functions that turn into commands without a framework getting in the way, it is.

## API Overview

### Commands

Register with `Command(signature, description, handler, opts...)`. The handler is any function returning a single `error`; positional arguments bind to its parameters in order.

```go
cli.Command("copy {src} {dst}", "Copy a file", func(src, dst string) error {
	return copyFile(src, dst)
})
```

The parameter type chooses how an argument behaves:

```go
func(name string)      // required:  {name}
func(name *string)     // optional:  nil when omitted
func(items ...string)  // repeated:  collects everything left over
```

Defaults and per-argument help live in the signature, or come from `ArgDesc`:

```go
cli.Command("sleep {seconds=1}", "Sleep a while",
	func(seconds string) error { return nil })

cli.Command("greet {name:person to greet}", "Greet someone",
	func(name string) error { return nil })

cli.Command("greet {name}", "Greet someone",
	func(name string) error { return nil },
	cli.ArgDesc("name", "person to greet"),
)
```

A signature with no command name (e.g. `"{file}"`) registers a **root command** that runs when no subcommand is given.

### Flags

Declare a struct, tag the fields, and take it as the handler's first parameter. Supported field types are `bool`, `string`, and `int`.

```go
type flags struct {
	Verbose bool   `flag:"verbose,v" desc:"print extra detail"`
	Output  string `flag:"output,o" default:"-" desc:"output file"`
	Level   int    `flag:"level,l" count:"true" desc:"raise the log level"`
}
```

| tag | meaning |
|---|---|
| `flag:"long,short"` | long name (required) and an optional one-character short name |
| `default:"value"` | value used when the flag is absent |
| `desc:"text"` | help text |
| `count:"true"` | repeatable int flag: `-lll` or `-l -l -l` counts to 3 |

Every exported field must carry a `flag` tag. Booleans can be bare or explicit:

```sh
app run --verbose          # true
app run --verbose=false    # explicit
app run -vvv               # count flag → 3
app run -o out.txt -l -l   # short forms, repeated
```

The flags parameter is optional. Leave it off and flags are still parsed and validated; your handler just doesn't see them.

### Groups

`Group` nests related commands under a shared name.

```go
cli.Group("calc", "Arithmetic commands", func(g cli.GroupAdder) {
	g.Command("add {a} {b}", "Add two numbers", add)
	g.Command("sub {a} {b}", "Subtract two numbers", sub)
})
```

```sh
app calc add 1 2
app help calc
app help calc add
```

### Help and Version

`help` and `version` are added automatically unless you register commands by those names. Set the metadata they display:

```go
cli.Name("myapp")
cli.Description("Does useful things.")
cli.Version("1.2.3")
cli.Build("1.2.3", "abc123", "2026-05-07") // version, commit, build date
```

### Input and Output

Reach for `cli.In()`, `cli.Out()`, and `cli.Err()` instead of `os.Stdin`/`Stdout`/`Stderr` so commands stay testable.

```go
func handler() error {
	_, err := io.Copy(cli.Out(), cli.In())
	return err
}
```

## Recipes

### Scoped, composed flags

Flags declared on a command embed the group's flags, which embed the global flags. Embedding is the link: a more specific struct simply contains the broader one, and a handler that asks for `upperFlags` can read every flag in the chain.

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

	cli.Group("format", "Format text", func(g cli.GroupAdder) {
		g.Command("upper {text}", "Uppercase text", upper, cli.Flags[upperFlags]())
	}, cli.Flags[formatFlags]())

	cli.Run()
}

func upper(f upperFlags, text string) error {
	if f.Verbose { // reached through the embedded globalFlags
		fmt.Fprintln(cli.Err(), "upcasing", text)
	}
	out := f.Prefix + strings.ToUpper(text)
	if f.Strong {
		out += "!"
	}
	fmt.Fprintln(cli.Out(), out)
	return nil
}
```

### Middleware

Middleware wraps execution as `func(next func() error) error`. Register it globally with `Use`, or on a group or command with the `Middleware` option. Returning before calling `next` stops the command from running.

```go
func timed(next func() error) error {
	start := time.Now()
	err := next()
	fmt.Fprintf(cli.Err(), "took %s\n", time.Since(start))
	return err
}

cli.Use(timed)
cli.Group("db", "Database commands", register, cli.Middleware(requireConnection))
cli.Command("risky", "Do something risky", run, cli.Middleware(confirm))
```

### Hidden commands

`Hidden()` keeps a command or group out of help output; `HiddenWhen(fn)` hides it based on runtime state, re-evaluated each time help renders. Both stay fully executable.

```go
cli.Command("debug", "Internal debugging", debug, cli.Hidden())
cli.Command("beta", "Experimental feature", beta,
	cli.HiddenWhen(func() bool { return !betaEnabled() }))
```

### Pass-through arguments

`RawArgs()` forwards everything after the command name to a `func(...string) error` handler untouched, with no flag parsing or `--` terminator. Useful for wrapping other tools.

```go
cli.Command("exec {args}", "Run a command verbatim", func(args ...string) error {
	return runExternal(args)
}, cli.RawArgs())
```

### Testing commands

`RunWith` runs the CLI with an explicit argument list, and the I/O writers are swappable, so a command test needs no subprocess.

```go
func TestGreet(t *testing.T) {
	var out bytes.Buffer
	defer cli.SetOut(cli.SetOut(&out)) // swap in, restore the previous writer

	cli.Command("greet {name}", "Greet someone", func(name string) error {
		fmt.Fprintf(cli.Out(), "Hello, %s\n", name)
		return nil
	})

	if err := cli.RunWith([]string{"app", "greet", "Alice"}); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "Hello, Alice\n" {
		t.Fatalf("got %q", got)
	}
}
```

Registration is package-global, so tests that register commands should reset or isolate that state between runs.

## Examples

Runnable programs live in [`examples/`](examples/):

- [`examples/cat`](examples/cat): a root command using the I/O helpers.
- [`examples/math`](examples/math): named commands and a group.
- [`examples/text`](examples/text): scoped flags composed by embedding.
- [`examples/sleep`](examples/sleep): middleware on groups and commands.

## Contributing

Run `go test ./...` before opening a PR. For anything beyond a small fix, open an issue first.

## License

MIT. See [LICENSE](LICENSE).
