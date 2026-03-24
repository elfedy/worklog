package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gen2brain/beeep"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: %s start <duration>\n", os.Args[0])
		fmt.Fprintln(os.Stderr, `example: timer start 25m`)
		os.Exit(1)
	}

	switch os.Args[1] {
	case "start":
		startFlags := flag.NewFlagSet("start", flag.ExitOnError)
		title := startFlags.String("title", "Timer finished", "notification title")
		message := startFlags.String("message", "Time is up.", "notification message")
		startFlags.Parse(os.Args[2:])

		if startFlags.NArg() != 1 {
			fmt.Fprintf(os.Stderr, "usage: %s start <duration>\n", os.Args[0])
			os.Exit(1)
		}

		duration, err := time.ParseDuration(strings.TrimSpace(startFlags.Arg(0)))
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid duration %q: %v\n", startFlags.Arg(0), err)
			os.Exit(1)
		}

		if duration <= 0 {
			fmt.Fprintln(os.Stderr, "duration must be greater than zero")
			os.Exit(1)
		}

		ticker := time.NewTicker(time.Second)
		done := time.NewTimer(duration)
		endAt := time.Now().Add(duration)
		interrupts := make(chan os.Signal, 1)
		signal.Notify(interrupts, os.Interrupt, syscall.SIGTERM)

		defer ticker.Stop()
		defer done.Stop()
		defer signal.Stop(interrupts)

		fmt.Printf("Timer started for %s\n", duration.Round(time.Second))
		fmt.Print("\rRemaining: ", duration.Round(time.Second))

		for {
			select {
			case <-interrupts:
				fmt.Print("\rTimer cancelled.                     \n")
				os.Exit(130)
			case <-ticker.C:
				remaining := time.Until(endAt).Round(time.Second)
				if remaining < 0 {
					remaining = 0
				}

				fmt.Printf("\rRemaining: %-24s", remaining)
			case <-done.C:
				fmt.Print("\rRemaining: 0s                      \n")

				notifyErr := beeep.Notify(*title, *message, "")
				if notifyErr != nil {
					fmt.Fprintf(os.Stderr, "notification failed: %v\n", notifyErr)
				}

				fmt.Printf("%s: %s\n", *title, *message)
				return
			}
		}
	}

	fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
	fmt.Fprintf(os.Stderr, "usage: %s start <duration>\n", os.Args[0])
	os.Exit(1)
}
