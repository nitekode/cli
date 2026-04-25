package cli

import "testing"

func TestParseSignatureValid(t *testing.T) {
	sig, err := parseSignature("greet {arg1} {arg2=default} {arg3}")
	if err != nil {
		t.Fatalf("parseSignature returned error: %v", err)
	}

	if sig.Command != "greet" {
		t.Fatalf("Command = %q, want %q", sig.Command, "greet")
	}

	if len(sig.Args) != 3 {
		t.Fatalf("len(Args) = %d, want 3", len(sig.Args))
	}

	if sig.Args[0] != (argument{Name: "arg1"}) {
		t.Fatalf("Args[0] = %#v", sig.Args[0])
	}

	if sig.Args[1] != (argument{
		Name:       "arg2",
		HasDefault: true,
		Default:    "default",
	}) {
		t.Fatalf("Args[1] = %#v", sig.Args[1])
	}

	if sig.Args[2] != (argument{Name: "arg3"}) {
		t.Fatalf("Args[2] = %#v", sig.Args[2])
	}
}

func TestParseSignatureInvalid(t *testing.T) {
	tests := []struct {
		name string
		sig  string
	}{
		{
			name: "legacy optional syntax",
			sig:  "greet {arg1?}",
		},
		{
			name: "legacy repeated syntax",
			sig:  "greet {arg1*}",
		},
		{
			name: "legacy optional repeated syntax",
			sig:  "greet {arg1?*}",
		},
		{
			name: "legacy variadic syntax",
			sig:  "greet {arg1...}",
		},
		{
			name: "duplicate names",
			sig:  "greet {arg1} {arg1=default}",
		},
		{
			name: "empty argument",
			sig:  "greet {}",
		},
		{
			name: "invalid argument name",
			sig:  "greet {1arg}",
		},
		{
			name: "missing braces",
			sig:  "greet arg1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseSignature(tt.sig); err == nil {
				t.Fatalf("parseSignature(%q) returned nil error", tt.sig)
			}
		})
	}
}
