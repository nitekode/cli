package main

import (
	"fmt"
	"strconv"

	"github.com/nitekode/cli"
)

func main() {
	cli.Name("Math")
	cli.Version("1.0")

	cli.Command("pi", "Print the value of pi", func() error {
		fmt.Fprintln(cli.Out(), "3.14159")
		return nil
	})

	cli.Group("calc", "Calculator commands", func(ga cli.GroupAdder) {
		ga.Command("add {a} {b}", "Add two numbers", addHandler)
		ga.Command("sub {a} {b}", "Subtract two numbers", subHandler)
	})

	cli.Run()
}

func addHandler(a string, b string) error {
	numA, err := strconv.Atoi(a)
	if err != nil {
		return fmt.Errorf("could not convert %s to an int", a)
	}
	numB, err := strconv.Atoi(b)
	if err != nil {
		return fmt.Errorf("could not convert %s to an int", b)
	}

	fmt.Fprintf(cli.Out(), "%d + %d = %d\n", numA, numB, numA+numB)

	return nil

}

func subHandler(a string, b string) error {
	numA, err := strconv.Atoi(a)
	if err != nil {
		return fmt.Errorf("could not convert %s to an int", a)
	}
	numB, err := strconv.Atoi(b)
	if err != nil {
		return fmt.Errorf("could not convert %s to an int", b)
	}

	fmt.Fprintf(cli.Out(), "%d - %d = %d\n", numA, numB, numA-numB)

	return nil

}
