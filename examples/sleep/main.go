package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/nitekode/cli"
)

func main() {
	cli.Name("Sleep")
	cli.Version("1.0")
	cli.Use(uppercaser)

	cli.Group("duration", func(ga cli.GroupAdder) {
		ga.Command("second {duration=1}", secondCmdHandler)
		ga.Command("custom", customCmdHandler, cli.Middleware(hasDuration))
	}, cli.Middleware(sleepTimer))

	cli.Run()
}

func secondCmdHandler(duration string) error {
	dur, _ := strconv.Atoi(duration)

	fmt.Fprintln(cli.Out(), "Going to sleep")
	time.Sleep(time.Duration(dur) * time.Second)
	fmt.Fprintln(cli.Out(), "Waking up")

	return nil
}

func customCmdHandler() error {
	durEnv, _ := os.LookupEnv("DURATION")
	dur, _ := strconv.Atoi(durEnv)

	fmt.Fprintln(cli.Out(), "Going to sleep")
	time.Sleep(time.Duration(dur) * time.Second)
	fmt.Fprintln(cli.Out(), "Waking up")

	return nil
}

func uppercaser(next func() error) error {
	// Capture the output from the commands and groups
	var b bytes.Buffer
	prevOut := cli.SetOut(&b)

	// Call next middleware
	err := next()

	// Restore the default output, uppercase all the captured output and
	// print it
	cli.SetOut(prevOut)
	fmt.Fprint(cli.Out(), strings.ToUpper(b.String()))

	return err
}

func sleepTimer(next func() error) error {
	start := time.Now()

	err := next()
	if err != nil {
		// Something went wrong in one of the commands, so dont print how long
		// we waited
		return err
	}

	fmt.Fprintf(cli.Out(), "Slept for %s\n", time.Since(start).Round(time.Millisecond))

	return nil
}

func hasDuration(next func() error) error {
	// Business logic like this should be in the command but we put it in a
	// middleware here for demonstration purposes

	dur, found := os.LookupEnv("DURATION")
	if !found {
		return errors.New("DURATION env variable must be set")
	}
	if _, err := strconv.Atoi(dur); err != nil {
		return errors.New("DURATION must be set to an integer")
	}

	return next()
}
