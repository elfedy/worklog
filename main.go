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
	case "summary":
		runSummary(os.Args[2:])
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
	fmt.Println("  summary Show entries for a period or date range, optionally filtered by text")
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
	entriesDir := resolveEntriesDir(worklogDir, config)

	todayEntriesPath := filepath.Join(entriesDir, time.Now().Format("2006-01-02")+".json")
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

	runTimeblock(reader, entriesDir, goal, durationMinutes)
}

func runResume() {
	reader := bufio.NewReader(os.Stdin)

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
	entriesDir := resolveEntriesDir(worklogDir, config)
	entry, entryErr := lastEntry(entriesDir)
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

	runTimeblock(reader, entriesDir, entry.Goal, remainingMinutes)
}

func runTimeblock(reader *bufio.Reader, entriesDir string, goal string, durationMinutes int) {
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
			endedAt := time.Now()
			elapsedMinutes := int(endedAt.Sub(startedAt) / time.Minute)

			fmt.Printf("\rTimeblock interrupted at %d minutes.               \n", elapsedMinutes)

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

			entriesPath := filepath.Join(entriesDir, startedAt.Format("2006-01-02")+".json")
			if saveErr := saveEntry(entriesDir, startedAt, entry); saveErr != nil {
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

			entriesPath := filepath.Join(entriesDir, startedAt.Format("2006-01-02")+".json")
			if saveErr := saveEntry(entriesDir, startedAt, entry); saveErr != nil {
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
	entriesDir := resolveEntriesDir(worklogDir, config)

	today := time.Now()
	todayEntriesPath := filepath.Join(entriesDir, today.Format("2006-01-02")+".json")
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
	printEntries(todayEntries, false)
}

func runSummary(args []string) {
	homeDir, homeErr := os.UserHomeDir()
	if homeErr != nil {
		fmt.Fprintf(os.Stderr, "failed to resolve home directory: %v\n", homeErr)
		os.Exit(1)
	}

	if len(args) < 1 || len(args) > 3 {
		fmt.Fprintln(os.Stderr, "usage: worklog summary <week|month|year> [filter] or worklog summary <YYYY-MM-DD> <YYYY-MM-DD> [filter]")
		os.Exit(1)
	}

	worklogDir := filepath.Join(homeDir, ".worklog")
	config, configErr := loadWorklogConfig(worklogDir)
	if configErr != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", configErr)
		os.Exit(1)
	}
	entriesDir := resolveEntriesDir(worklogDir, config)
	start, end, label, filter, parseErr := parseSummaryArgs(args, time.Now())
	if parseErr != nil {
		fmt.Fprintf(os.Stderr, "%v\n", parseErr)
		os.Exit(1)
	}

	allEntries, readErr := readAllEntries(entriesDir)
	if readErr != nil {
		fmt.Fprintf(os.Stderr, "failed to read entries: %v\n", readErr)
		os.Exit(1)
	}

	entries := filterEntriesByTimeRange(allEntries, start, end)
	entries = filterEntriesByText(entries, filter)
	totalMinutes := 0
	for _, entry := range entries {
		totalMinutes += entry.DurationMinutes
	}

	fmt.Printf("Summary for %s\n", label)
	if filter != "" {
		fmt.Printf("Filter: %q\n", filter)
	}
	fmt.Printf("Total: %d minutes across %d entries\n", totalMinutes, len(entries))

	if len(entries) == 0 {
		fmt.Println("No entries found for that period.")
		return
	}

	fmt.Println()
	printEntries(entries, true)
}

func parseSummaryArgs(args []string, now time.Time) (time.Time, time.Time, string, string, error) {
	location := now.Location()

	period := args[0]
	switch period {
	case "week":
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}

		start := time.Date(now.Year(), now.Month(), now.Day()-(weekday-1), 0, 0, 0, 0, location)
		end := start.AddDate(0, 0, 7)
		filter := ""
		if len(args) > 1 {
			filter = args[1]
		}

		return start, end, fmt.Sprintf("week starting %s", start.Format("2006-01-02")), filter, nil
	case "month":
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, location)
		end := start.AddDate(0, 1, 0)
		filter := ""
		if len(args) > 1 {
			filter = args[1]
		}

		return start, end, start.Format("January 2006"), filter, nil
	case "year":
		start := time.Date(now.Year(), time.January, 1, 0, 0, 0, 0, location)
		end := start.AddDate(1, 0, 0)
		filter := ""
		if len(args) > 1 {
			filter = args[1]
		}

		return start, end, start.Format("2006"), filter, nil
	}

	start, startErr := time.ParseInLocation("2006-01-02", args[0], location)
	if startErr != nil {
		return time.Time{}, time.Time{}, "", "", fmt.Errorf("invalid period %q, expected week, month, year, or two ISO dates", period)
	}

	if len(args) < 2 {
		return time.Time{}, time.Time{}, "", "", fmt.Errorf("missing end date, expected worklog summary <YYYY-MM-DD> <YYYY-MM-DD> [filter]")
	}

	endDate, endErr := time.ParseInLocation("2006-01-02", args[1], location)
	if endErr != nil {
		return time.Time{}, time.Time{}, "", "", fmt.Errorf("invalid end date %q, expected YYYY-MM-DD", args[1])
	}

	if endDate.Before(start) {
		return time.Time{}, time.Time{}, "", "", fmt.Errorf("end date %s cannot be before start date %s", args[1], args[0])
	}

	filter := ""
	if len(args) > 2 {
		filter = args[2]
	}

	return start, endDate.AddDate(0, 0, 1), fmt.Sprintf("%s to %s", args[0], args[1]), filter, nil
}

func printEntries(entries []timeblockEntry, includeDate bool) {
	for index, entry := range entries {
		entryHeader := fmt.Sprintf("#%d", index+1)
		if includeDate {
			entryHeader = fmt.Sprintf("%s | %s", entryHeader, entry.StartedAt.Local().Format("2006-01-02"))
		}

		fmt.Printf(
			"\n%s\n %s-%s | %s | %s\nresult:\n %s\n",
			entryHeader,
			entry.StartedAt.Local().Format("15:04"),
			entry.EndedAt.Local().Format("15:04"),
			entry.DurationLabel,
			entry.Goal,
			entry.Result,
		)
	}
}
