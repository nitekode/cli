package main

import (
	"errors"
	"io"
	"os"

	"github.com/nitekode/cli"
)

func main() {
	cli.Command("{file}", catHandler)
	cli.Run()
}

func catHandler(file *string) error {
	if file == nil || *file == "-" {
		_, err := io.Copy(cli.Out(), cli.In())
		return err
	}

	if _, err := os.Stat(*file); errors.Is(err, os.ErrNotExist) {
		return errors.New("No such file or directory")
	}

	r, err := os.Open(*file)
	if err != nil {
		return err
	}
	defer r.Close()

	_, err = io.Copy(cli.Out(), r)
	return err
}
