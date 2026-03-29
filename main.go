package main

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gen2brain/beeep"
)

func main() {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Timeblock")

	fmt.Print("What do you want to achieve in this timeblock? ")
	goal, err := reader.ReadString('\n')
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read goal: %v\n", err)
		os.Exit(1)
	}

	goal = strings.TrimSpace(goal)
	if goal == "" {
		fmt.Fprintln(os.Stderr, "goal cannot be empty")
		os.Exit(1)
	}

	homeDir, homeErr := os.UserHomeDir()
	if homeErr != nil {
		fmt.Fprintf(os.Stderr, "failed to resolve home directory: %v\n", homeErr)
		os.Exit(1)
	}

	worklogDir := filepath.Join(homeDir, ".worklog")
	config, configErr := loadWorklogConfig(worklogDir)
	if configErr != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", configErr)
		os.Exit(1)
	}

	todayEntriesPath := filepath.Join(worklogDir, "entries", time.Now().Format("2006-01-02")+".json")
	todayEntries, todayEntriesErr := readEntries(todayEntriesPath)
	if todayEntriesErr != nil {
		fmt.Fprintf(os.Stderr, "failed to read %s: %v\n", todayEntriesPath, todayEntriesErr)
		os.Exit(1)
	}

	todayMinutes := 0
	for _, entry := range todayEntries {
		todayMinutes += entry.DurationMinutes
	}

	fmt.Println()
	fmt.Printf("Today: %d/%d minutes worked\n", todayMinutes, config.MinutesPerDay)
	fmt.Println("How long do you want to focus?")
	for index, minutes := range config.TimeSets {
		fmt.Printf("%d. %d minutes\n", index+1, minutes)
	}
	fmt.Printf("%d. Custom\n", len(config.TimeSets)+1)
	fmt.Print("Choose an option: ")

	choiceInput, err := reader.ReadString('\n')
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read duration choice: %v\n", err)
		os.Exit(1)
	}

	choice, err := strconv.Atoi(strings.TrimSpace(choiceInput))
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid choice %q\n", strings.TrimSpace(choiceInput))
		os.Exit(1)
	}

	durationMinutes := 0
	durationLabel := ""

	if choice >= 1 && choice <= len(config.TimeSets) {
		durationMinutes = config.TimeSets[choice-1]
		durationLabel = fmt.Sprintf("%d minutes", durationMinutes)
	} else if choice == len(config.TimeSets)+1 {
		fmt.Print("Enter custom minutes: ")

		customInput, customErr := reader.ReadString('\n')
		if customErr != nil {
			fmt.Fprintf(os.Stderr, "failed to read custom minutes: %v\n", customErr)
			os.Exit(1)
		}

		customMinutes, customParseErr := strconv.Atoi(strings.TrimSpace(customInput))
		if customParseErr != nil {
			fmt.Fprintf(os.Stderr, "invalid custom minutes %q\n", strings.TrimSpace(customInput))
			os.Exit(1)
		}

		if customMinutes <= 0 {
			fmt.Fprintln(os.Stderr, "custom minutes must be greater than zero")
			os.Exit(1)
		}

		durationMinutes = customMinutes
		durationLabel = fmt.Sprintf("%d minutes", durationMinutes)
	} else {
		fmt.Fprintf(os.Stderr, "choice must be between 1 and %d\n", len(config.TimeSets)+1)
		os.Exit(1)
	}

	duration := time.Duration(durationMinutes) * time.Minute
	startedAt := time.Now()
	endAt := startedAt.Add(duration)
	ticker := time.NewTicker(time.Second)
	done := time.NewTimer(duration)
	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, os.Interrupt, syscall.SIGTERM)

	defer ticker.Stop()
	defer done.Stop()
	defer signal.Stop(interrupts)

	fmt.Println()
	fmt.Printf("Starting %s timeblock for: %s\n", durationLabel, goal)
	fmt.Printf("Ends at %s\n", endAt.Format("15:04"))
	fmt.Print("\rRemaining: ", duration.Round(time.Second))

	for {
		select {
		case <-interrupts:
			fmt.Print("\rTimeblock interrupted.               \n")

			fmt.Println()
			fmt.Print("Why was this interrupted? ")

			result, readErr := reader.ReadString('\n')
			if readErr != nil {
				fmt.Fprintf(os.Stderr, "failed to read interruption reason: %v\n", readErr)
				os.Exit(1)
			}

			result = strings.TrimSpace(result)
			if result == "" {
				fmt.Fprintln(os.Stderr, "interruption reason cannot be empty")
				os.Exit(1)
			}

			endedAt := time.Now()
			elapsedMinutes := int(endedAt.Sub(startedAt) / time.Minute)

			entry := timeblockEntry{
				Goal:            goal,
				Result:          fmt.Sprintf("[INTERRUPTED %s] %s", durationLabel, result),
				DurationMinutes: elapsedMinutes,
				DurationLabel:   fmt.Sprintf("%d minutes", elapsedMinutes),
				StartedAt:       startedAt,
				EndedAt:         endedAt,
			}

			entriesPath := filepath.Join(worklogDir, "entries", startedAt.Format("2006-01-02")+".json")
			if saveErr := saveEntry(worklogDir, startedAt, entry); saveErr != nil {
				fmt.Fprintf(os.Stderr, "%v\n", saveErr)
				os.Exit(1)
			}

			fmt.Printf("Saved interrupted entry to %s\n", entriesPath)
			return
		case <-ticker.C:
			remaining := time.Until(endAt).Round(time.Second)
			if remaining < 0 {
				remaining = 0
			}

			fmt.Printf("\rRemaining: %-24s", remaining)
		case <-done.C:
			fmt.Print("\rRemaining: 0s                      \n")

			notifyErr := beeep.Notify("Timeblock finished", goal, "")
			if notifyErr != nil {
				fmt.Fprintf(os.Stderr, "notification failed: %v\n", notifyErr)
			}

			fmt.Println()
			fmt.Print("What was the result? ")

			result, readErr := reader.ReadString('\n')
			if readErr != nil {
				fmt.Fprintf(os.Stderr, "failed to read result: %v\n", readErr)
				os.Exit(1)
			}

			result = strings.TrimSpace(result)
			if result == "" {
				fmt.Fprintln(os.Stderr, "result cannot be empty")
				os.Exit(1)
			}

			entry := timeblockEntry{
				Goal:            goal,
				Result:          result,
				DurationMinutes: durationMinutes,
				DurationLabel:   durationLabel,
				StartedAt:       startedAt,
				EndedAt:         time.Now(),
			}

			entriesPath := filepath.Join(worklogDir, "entries", startedAt.Format("2006-01-02")+".json")
			if saveErr := saveEntry(worklogDir, startedAt, entry); saveErr != nil {
				fmt.Fprintf(os.Stderr, "%v\n", saveErr)
				os.Exit(1)
			}

			fmt.Printf("Saved entry to %s\n", entriesPath)
			return
		}
	}
}
