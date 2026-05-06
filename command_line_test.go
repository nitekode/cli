package cli

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestParseCommandLine(t *testing.T) {
	tests := []struct {
		name            string
		args            []string
		booleanFlags    []string
		wantPositionals []string
		wantFlags       map[string]bool
		wantOptions     map[string][]string
	}{
		{
			name:            "positionals only",
			args:            []string{"hello", "world"},
			wantPositionals: []string{"hello", "world"},
			wantFlags:       map[string]bool{},
			wantOptions:     map[string][]string{},
		},
		{
			name:            "long boolean flag",
			args:            []string{"--verbose", "file.txt"},
			booleanFlags:    []string{"verbose"},
			wantPositionals: []string{"file.txt"},
			wantFlags:       map[string]bool{"verbose": true},
			wantOptions:     map[string][]string{},
		},
		{
			name:            "long boolean flag with explicit false",
			args:            []string{"--verbose=false", "file.txt"},
			booleanFlags:    []string{"verbose"},
			wantPositionals: []string{"file.txt"},
			wantFlags:       map[string]bool{"verbose": false},
			wantOptions:     map[string][]string{},
		},
		{
			name:            "long options with equals and next value",
			args:            []string{"--profile=prod", "--output", "result.txt", "input.txt"},
			wantPositionals: []string{"input.txt"},
			wantFlags:       map[string]bool{},
			wantOptions: map[string][]string{
				"profile": {"prod"},
				"output":  {"result.txt"},
			},
		},
		{
			name:            "explicit empty option value",
			args:            []string{"--prefix=", "text"},
			wantPositionals: []string{"text"},
			wantFlags:       map[string]bool{},
			wantOptions:     map[string][]string{"prefix": {""}},
		},
		{
			name:            "short boolean flag",
			args:            []string{"-v", "file.txt"},
			booleanFlags:    []string{"v"},
			wantPositionals: []string{"file.txt"},
			wantFlags:       map[string]bool{"v": true},
			wantOptions:     map[string][]string{},
		},
		{
			name:            "short option with equals and next value",
			args:            []string{"-p=prod", "-o", "result.txt", "input.txt"},
			wantPositionals: []string{"input.txt"},
			wantFlags:       map[string]bool{},
			wantOptions: map[string][]string{
				"p": {"prod"},
				"o": {"result.txt"},
			},
		},
		{
			name:            "bundled short boolean flags",
			args:            []string{"-abc", "file.txt"},
			booleanFlags:    []string{"a", "b", "c"},
			wantPositionals: []string{"file.txt"},
			wantFlags:       map[string]bool{"a": true, "b": true, "c": true},
			wantOptions:     map[string][]string{},
		},
		{
			name:            "repeated options",
			args:            []string{"--tag", "one", "--tag=two"},
			wantPositionals: []string{},
			wantFlags:       map[string]bool{},
			wantOptions:     map[string][]string{"tag": {"one", "two"}},
		},
		{
			name:            "double dash stops parsing named inputs",
			args:            []string{"--verbose", "--", "--profile", "prod"},
			booleanFlags:    []string{"verbose"},
			wantPositionals: []string{"--profile", "prod"},
			wantFlags:       map[string]bool{"verbose": true},
			wantOptions:     map[string][]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCommandLine(tt.args, tt.booleanFlags)
			if err != nil {
				t.Fatalf("parseCommandLine returned error: %v", err)
			}

			if !slices.Equal(got.positionals, tt.wantPositionals) {
				t.Fatalf("positionals = %#v, want %#v", got.positionals, tt.wantPositionals)
			}
			if !reflect.DeepEqual(got.flags, tt.wantFlags) {
				t.Fatalf("flags = %#v, want %#v", got.flags, tt.wantFlags)
			}
			if !reflect.DeepEqual(got.options, tt.wantOptions) {
				t.Fatalf("options = %#v, want %#v", got.options, tt.wantOptions)
			}
		})
	}
}

func TestParseCommandLineErrors(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		booleanFlags []string
		wantErr      string
	}{
		{
			name:         "invalid boolean value",
			args:         []string{"--verbose=maybe"},
			booleanFlags: []string{"verbose"},
			wantErr:      `flag "verbose" expects true or false`,
		},
		{
			name:         "explicit empty boolean value",
			args:         []string{"--verbose="},
			booleanFlags: []string{"verbose"},
			wantErr:      `flag "verbose" expects true or false`,
		},
		{
			name:    "missing long option value",
			args:    []string{"--profile"},
			wantErr: `option "profile" expects a value`,
		},
		{
			name:    "missing short option value",
			args:    []string{"-p"},
			wantErr: `option "p" expects a value`,
		},
		{
			name:    "next long named input is not option value",
			args:    []string{"--profile", "--verbose"},
			wantErr: `option "profile" expects a value`,
		},
		{
			name:    "next short named input is not option value",
			args:    []string{"--profile", "-v"},
			wantErr: `option "profile" expects a value`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseCommandLine(tt.args, tt.booleanFlags)
			if err == nil {
				t.Fatal("parseCommandLine returned nil error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestCommandLineValidate(t *testing.T) {
	tests := []struct {
		name              string
		parsed            commandLine
		flags             *flagSet
		expectedArguments []commandArgument
		wantFlags         map[string]bool
		wantOptions       map[string][]string
	}{
		{
			name: "validates flags options and arguments",
			parsed: commandLine{
				positionals: []string{"alice"},
				flags:       map[string]bool{"verbose": true},
				options:     map[string][]string{"profile": {"prod"}},
			},
			flags: testCommandLineFlagSet(),
			expectedArguments: []commandArgument{
				{Name: "name", Kind: requiredArgument},
			},
			wantFlags:   map[string]bool{"verbose": true},
			wantOptions: map[string][]string{"profile": {"prod"}},
		},
		{
			name: "normalizes short names",
			parsed: commandLine{
				positionals: []string{"alice"},
				flags:       map[string]bool{"v": true},
				options:     map[string][]string{"p": {"prod"}, "profile": {"dev"}},
			},
			flags: testCommandLineFlagSet(),
			expectedArguments: []commandArgument{
				{Name: "name", Kind: requiredArgument},
			},
			wantFlags:   map[string]bool{"verbose": true},
			wantOptions: map[string][]string{"profile": {"dev", "prod"}},
		},
		{
			name: "allows optional and default arguments to be omitted",
			parsed: commandLine{
				positionals: []string{"alice"},
				flags:       map[string]bool{},
				options:     map[string][]string{},
			},
			expectedArguments: []commandArgument{
				{Name: "name", Kind: requiredArgument},
				{Name: "title", Kind: optionalArgument},
				{Name: "suffix", Kind: defaultArgument},
			},
			wantFlags:   map[string]bool{},
			wantOptions: map[string][]string{},
		},
		{
			name: "allows repeated arguments",
			parsed: commandLine{
				positionals: []string{"one", "two", "three"},
				flags:       map[string]bool{},
				options:     map[string][]string{},
			},
			expectedArguments: []commandArgument{
				{Name: "values", Kind: repeatedArgument},
			},
			wantFlags:   map[string]bool{},
			wantOptions: map[string][]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.parsed.validate(tt.flags, tt.expectedArguments)
			if err != nil {
				t.Fatalf("validate returned error: %v", err)
			}
			if !reflect.DeepEqual(tt.parsed.flags, tt.wantFlags) {
				t.Fatalf("flags = %#v, want %#v", tt.parsed.flags, tt.wantFlags)
			}
			if !reflect.DeepEqual(tt.parsed.options, tt.wantOptions) {
				t.Fatalf("options = %#v, want %#v", tt.parsed.options, tt.wantOptions)
			}
		})
	}
}

func TestCommandLineValidateErrors(t *testing.T) {
	tests := []struct {
		name              string
		parsed            commandLine
		flags             *flagSet
		expectedArguments []commandArgument
		wantErr           string
	}{
		{
			name: "unknown flag",
			parsed: commandLine{
				flags:   map[string]bool{"debug": true},
				options: map[string][]string{},
			},
			flags:   testCommandLineFlagSet(),
			wantErr: `unknown flag "debug"`,
		},
		{
			name: "unknown option",
			parsed: commandLine{
				flags:   map[string]bool{},
				options: map[string][]string{"output": {"file.txt"}},
			},
			flags:   testCommandLineFlagSet(),
			wantErr: `unknown option "output"`,
		},
		{
			name: "option used as boolean flag",
			parsed: commandLine{
				flags:   map[string]bool{"profile": true},
				options: map[string][]string{},
			},
			flags:   testCommandLineFlagSet(),
			wantErr: `option "profile" used as boolean flag`,
		},
		{
			name: "flag used as value option",
			parsed: commandLine{
				flags:   map[string]bool{},
				options: map[string][]string{"verbose": {"true"}},
			},
			flags:   testCommandLineFlagSet(),
			wantErr: `flag "verbose" used as value option`,
		},
		{
			name: "missing required argument",
			parsed: commandLine{
				flags:       map[string]bool{},
				options:     map[string][]string{},
				positionals: []string{},
			},
			expectedArguments: []commandArgument{
				{Name: "name", Kind: requiredArgument},
			},
			wantErr: "missing arguments: got 0, want at least 1",
		},
		{
			name: "too many arguments",
			parsed: commandLine{
				flags:       map[string]bool{},
				options:     map[string][]string{},
				positionals: []string{"alice", "bob"},
			},
			expectedArguments: []commandArgument{
				{Name: "name", Kind: requiredArgument},
			},
			wantErr: "too many arguments: got 2, want at most 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.parsed.validate(tt.flags, tt.expectedArguments)
			if err == nil {
				t.Fatal("validate returned nil error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func testCommandLineFlagSet() *flagSet {
	return &flagSet{
		fields: []flagField{
			{Name: "verbose", Short: "v", Bool: true},
			{Name: "profile", Short: "p"},
		},
	}
}
