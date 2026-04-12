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
	if len(os.Args) < 2 {
		printHelp()
		return
	}

	switch os.Args[1] {
	case "start":
		runStart()
	case "resume":
		runResume()
	case "status":
		runStatus()
	case "help", "-h", "--help":
		printHelp()
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q\n\n", os.Args[1])
		printHelp()
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Println("worklog")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  worklog <command>")
	fmt.Println()
	fmt.Println("Available Commands:")
	fmt.Println("  start   Start a new timeblock")
	fmt.Println("  resume  Resume the last interrupted timeblock")
	fmt.Println("  status  Show current status")
	fmt.Println("  help    Show this help menu")
}

func runStart() {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Starting a Timeblock")

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

	fmt.Printf("Today: %d/%d minutes worked\n", todayMinutes, config.MinutesPerDay)

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

	if choice >= 1 && choice <= len(config.TimeSets) {
		durationMinutes = config.TimeSets[choice-1]
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
	} else {
		fmt.Fprintf(os.Stderr, "choice must be between 1 and %d\n", len(config.TimeSets)+1)
		os.Exit(1)
	}

	runTimeblock(reader, worklogDir, goal, durationMinutes)
}

func runResume() {
	reader := bufio.NewReader(os.Stdin)

	homeDir, homeErr := os.UserHomeDir()
	if homeErr != nil {
		fmt.Fprintf(os.Stderr, "failed to resolve home directory: %v\n", homeErr)
		os.Exit(1)
	}

	worklogDir := filepath.Join(homeDir, ".worklog")
	entry, entryErr := lastEntry(worklogDir)
	if entryErr != nil {
		fmt.Fprintf(os.Stderr, "failed to read last entry: %v\n", entryErr)
		os.Exit(1)
	}

	if entry == nil {
		fmt.Fprintln(os.Stderr, "no entries found")
		os.Exit(1)
	}

	if !entry.Interrupted {
		fmt.Fprintln(os.Stderr, "last entry was not interrupted")
		os.Exit(1)
	}

	if entry.PlannedDurationMinutes <= 0 {
		fmt.Fprintln(os.Stderr, "last interrupted entry is missing planned duration")
		os.Exit(1)
	}

	remainingMinutes := entry.PlannedDurationMinutes - entry.DurationMinutes
	if remainingMinutes <= 0 {
		fmt.Fprintln(os.Stderr, "last interrupted entry has no remaining time")
		os.Exit(1)
	}

	fmt.Println("Resuming a Timeblock")
	fmt.Printf("Goal: %s\n", entry.Goal)
	fmt.Printf("Remaining: %d minutes\n", remainingMinutes)

	runTimeblock(reader, worklogDir, entry.Goal, remainingMinutes)
}

func runTimeblock(reader *bufio.Reader, worklogDir string, goal string, durationMinutes int) {
	durationLabel := fmt.Sprintf("%d minutes", durationMinutes)
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
				Goal:                   goal,
				Result:                 result,
				Interrupted:            true,
				PlannedDurationMinutes: durationMinutes,
				DurationMinutes:        elapsedMinutes,
				DurationLabel:          fmt.Sprintf("%d minutes", elapsedMinutes),
				StartedAt:              startedAt,
				EndedAt:                endedAt,
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
				Goal:                   goal,
				Result:                 result,
				Interrupted:            false,
				PlannedDurationMinutes: durationMinutes,
				DurationMinutes:        durationMinutes,
				DurationLabel:          durationLabel,
				StartedAt:              startedAt,
				EndedAt:                time.Now(),
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

func runStatus() {
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

	today := time.Now()
	todayEntriesPath := filepath.Join(worklogDir, "entries", today.Format("2006-01-02")+".json")
	todayEntries, todayEntriesErr := readEntries(todayEntriesPath)
	if todayEntriesErr != nil {
		fmt.Fprintf(os.Stderr, "failed to read %s: %v\n", todayEntriesPath, todayEntriesErr)
		os.Exit(1)
	}

	todayMinutes := 0
	for _, entry := range todayEntries {
		todayMinutes += entry.DurationMinutes
	}

	fmt.Printf("Today: %d/%d minutes worked\n", todayMinutes, config.MinutesPerDay)

	if len(todayEntries) == 0 {
		fmt.Println("No entries logged today.")
		return
	}

	fmt.Println()
	fmt.Printf("Entries for %s:\n", today.Format("2006-01-02"))
	for index, entry := range todayEntries {
		fmt.Printf(
			"\n#%d\n %s-%s | %s | %s\nresult:\n %s\n",
			index+1,
			entry.StartedAt.Local().Format("15:04"),
			entry.EndedAt.Local().Format("15:04"),
			entry.DurationLabel,
			entry.Goal,
			entry.Result,
		)

	}
}
