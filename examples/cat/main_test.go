package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/nitekode/cli"
)

func TestCatFunctionReadsFromStdinWhenNoFileIsGiven(t *testing.T) {
	input := strings.NewReader("hello from stdin\n")
	var output bytes.Buffer
	previousIn := cli.SetIn(input)
	previousOut := cli.SetOut(&output)

	t.Cleanup(func() {
		cli.SetIn(previousIn)
		cli.SetOut(previousOut)
	})

	if err := catHandler(nil); err != nil {
		t.Fatalf("cat returned error: %v", err)
	}

	if output.String() != "hello from stdin\n" {
		t.Fatalf("output = %q, want %q", output.String(), "hello from stdin\n")
	}
}
